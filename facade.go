package webssh

import (
	"bufio"
	"context"
	"fmt"
	"github.com/youkale/webssh/logger"
	"net"
	"net/http"
	"strings"
)

const (
	NotFound = `HTTP/1.0 404 Not Found
Server: webs.sh
Content-Length: %d

Tunnel %s not found
`

	BadRequest = `HTTP/1.0 400 Bad Request
Server: webs.sh
Content-Length: 12

Bad Request
`
)

func badRequest(conn net.Conn) {
	conn.Write([]byte(BadRequest))
	conn.Close()
}

func notFound(id string, conn net.Conn) {
	conn.Write([]byte(fmt.Sprintf(NotFound, len(id)+18, id)))
	conn.Close()
}

func facadeServe(ctx context.Context, addr string, forward func(facadeId string, request *facadeRequest) bool) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("start Listen", err, map[string]interface{}{
			"module":  "facade",
			"address": addr,
		})
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			c, err := ln.Accept()
			if nil != err {
				logger.Error("start Accept", err, map[string]interface{}{
					"module":  "facade",
					"address": addr,
				})
				c.Close()
			} else {
				go func() {
					reader := newBufferedReader(c)
					req, err := http.ReadRequest(bufio.NewReader(reader))
					if err != nil {
						logger.Warn("bad request", map[string]interface{}{
							"module": "facade",
						})
						badRequest(c)
						return
					}

					domainSep := strings.Split(req.Host, ".")
					if len(domainSep) <= 1 {
						logger.Warn("bad request", map[string]interface{}{
							"module": "facade",
							"method": req.Method,
							"path":   req.URL.Path,
							"host":   req.Host,
						})
						badRequest(c)
						return
					}
					id := domainSep[0]

					canForward := forward(id, &facadeRequest{Conn: reader.toBufferedConn(c), request: req})

					if canForward {
						logger.Debug("found forward", map[string]interface{}{
							"module":   "facade",
							"method":   req.Method,
							"accessId": id,
							"path":     req.URL.Path,
						})
					} else {
						notFound(id, c)
						logger.Warn("not found forward", map[string]interface{}{
							"module":   "facade",
							"method":   req.Method,
							"accessId": id,
							"path":     req.URL.Path,
						})
						return
					}
				}()
			}
		}
	}
}
