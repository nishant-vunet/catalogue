package catalogue

import (
	"strings"
	"time"

	"github.com/go-kit/kit/log"
)

// LoggingMiddleware logs method calls, parameters, results, and elapsed time.
func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw loggingMiddleware) List(tags []string, order string, pageNum, pageSize int, traceID string) (socks []Sock, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "List",
			"tags", strings.Join(tags, ", "),
			"order", order,
			"pageNum", pageNum,
			"pageSize", pageSize,
			"result", len(socks),
			"err", err,
			"took", time.Since(begin),
			"traceID", traceID,
		)
	}(time.Now())
	return mw.next.List(tags, order, pageNum, pageSize,traceID)
}

func (mw loggingMiddleware) Count(tags []string,traceID string) (n int, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Count",
			"tags", strings.Join(tags, ", "),
			"result", n,
			"err", err,
			"took", time.Since(begin),
			"traceID", traceID,
		)
	}(time.Now())
	return mw.next.Count(tags,traceID)
}

func (mw loggingMiddleware) Get(id string,traceID string) (s Sock, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Get",
			"id", id,
			"sock", s.ID,
			"err", err,
			"took", time.Since(begin),
			"traceID", traceID,
		)
	}(time.Now())
	return mw.next.Get(id,traceID)
}

func (mw loggingMiddleware) Tags(traceID string) (tags []string, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Tags",
			"result", len(tags),
			"err", err,
			"took", time.Since(begin),
			"traceID", traceID,
		)
	}(time.Now())
	return mw.next.Tags(traceID)
}

func (mw loggingMiddleware) Health() (health []Health) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Health",
			"result", len(health),
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Health()
}
