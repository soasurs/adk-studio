package studio

import (
	"context"
	"errors"
	"net/http"
	"time"
)

func Serve(ctx context.Context, app *App, addr string) error {
	if addr == "" {
		addr = ":18080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           NewHandler(app),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
