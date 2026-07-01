package client

import (
    "context"
    "net"

    "github.com/rs/zerolog/log"
)

// ListenAndServe accepts client connections and invokes handler for each.
func ListenAndServe(ctx context.Context, addr string, handler func(context.Context, *ClientConn)) error {
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        return err
    }
    defer ln.Close()

    // Close the listener when the context is cancelled so the blocking Accept() below
    // returns instead of hanging — otherwise this goroutine never exits and graceful
    // shutdown times out. (l2go-rs3)
    go func() {
        <-ctx.Done()
        _ = ln.Close()
    }()

    log.Ctx(ctx).Info().Str("addr", addr).Msg("Game client listener started")

    for {
        c, err := ln.Accept()
        if err != nil {
            select {
            case <-ctx.Done():
                log.Ctx(ctx).Info().Msg("Game client listener stopping")
                return nil
            default:
                log.Ctx(ctx).Error().Err(err).Msg("accept failed")
                continue
            }
        }

        go func() {
            conn := NewClientConn(c)
            handler(ctx, conn)
            _ = conn.Close()
        }()
    }
}

