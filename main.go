package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	messageTimeFormat = "2006.01.02 15:04:05"
	_10Sec            = 10 * time.Second
)

type AlertTarget struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type AlertData struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Alert struct {
	Resolved   bool        `json:"resolved"`
	Level      string      `json:"level"`
	Target     AlertTarget `json:"target"`
	Data       AlertData   `json:"data"`
	ResolvedTS *int64      `json:"resolved_ts"`

	// unix time stamp
	TS int64 `json:"ts"`
}

func formatAlert(a Alert) string {
	ts := time.UnixMilli(a.TS)

	status := "Is not resolved"
	if a.Resolved {
		status = "Resolved"

		// when resolved ime stamp introduced this reformat status string
		// and add time stamp
		if a.ResolvedTS != nil {
			resolvedTS := time.UnixMilli(*a.ResolvedTS)
			prettyResolvedTS := resolvedTS.Format(messageTimeFormat)
			status = fmt.Sprintf("%s at *%s*", status, prettyResolvedTS)
		}
	}

	prettyData, err := json.MarshalIndent(a.Data.Data, "", "  ")
	if err != nil {
		prettyData = a.Data.Data
	}

	msgStrings := []string{
		fmt.Sprintf("## Alert %s", a.Data.Type),
		fmt.Sprintf("**Level**: %s", a.Level),
		fmt.Sprintf("**At**: *%s*", ts.Format(messageTimeFormat)),
		fmt.Sprintf("**Status**: %s", status),
		fmt.Sprintf("**%s**: `%s`", a.Target.Type, a.Target.ID),
		"```json",
		string(prettyData),
		"```",
	}

	return strings.Join(msgStrings, "\n\n")
}

func sendMessage(ctx context.Context, token string, chatID int64, msg string) error {

	body, err := json.Marshal(map[string]any{
		"chat_id":      chatID,
		"rich_message": map[string]string{"markdown": msg},
	})
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendRichMessage", token)

	ctx, cancel := context.WithTimeout(ctx, _10Sec)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", stripURLErr(err))
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram sendRichMessage: %w", stripURLErr(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// TODO: retry on 5xx and 429
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	return nil
}

func stripURLErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("url problem: %w", ue.Err)
	}
	return err
}

type server struct {
	token  string
	chatID int64
	logger *slog.Logger
}

func (h *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	const _1mb = 2 << 10

	body := http.MaxBytesReader(w, r.Body, _1mb)
	defer body.Close()

	data, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("read request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var alert Alert
	if err := json.Unmarshal(data, &alert); err != nil {
		h.logger.Error("json decoding", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := sendMessage(r.Context(), h.token, h.chatID, formatAlert(alert)); err != nil {
		h.logger.Error("send message", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {

	// logging
	logTextHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(logTextHandler)

	// get token
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return fmt.Errorf("BOT_TOKEN required")
	}

	// get chat id
	chatIDString := os.Getenv("CHAT_ID")
	if chatIDString == "" {
		return fmt.Errorf("CHAT_ID required")
	}

	chatID, err := strconv.ParseInt(chatIDString, 10, 64)
	if err != nil {
		return fmt.Errorf("CHAT_ID must be integer")
	}

	// context with sigterm await
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	// server
	srv := http.Server{
		Addr: ":9000",
		Handler: &server{
			token:  token,
			chatID: chatID,
			logger: logger,
		},
	}

	// run server
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server error", "error", err)
			stop()
		}
	}()

	logger.Warn("alerter started")

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), _10Sec)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	logger.Warn("alerter shutdown")

	return nil
}
