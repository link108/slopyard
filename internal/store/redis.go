package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrRateLimited = errors.New("rate limited")

type Limiter interface {
	CheckGlobal(ctx context.Context, fingerprint string) error
	ReserveHost(ctx context.Context, fingerprint string, host string) (func(context.Context), error)
	Close() error
}

type AllowAllLimiter struct{}

func (AllowAllLimiter) CheckGlobal(context.Context, string) error {
	return nil
}

func (AllowAllLimiter) ReserveHost(context.Context, string, string) (func(context.Context), error) {
	return func(context.Context) {}, nil
}

func (AllowAllLimiter) Close() error {
	return nil
}

type RedisLimiter struct {
	client       *redis.Client
	globalLimit  int
	globalWindow time.Duration
	hostWindow   time.Duration
}

func NewRedisLimiter(redisURL string, globalLimit int, globalWindow time.Duration, hostWindow time.Duration) (*RedisLimiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, err
	}
	return &RedisLimiter{
		client:       client,
		globalLimit:  globalLimit,
		globalWindow: globalWindow,
		hostWindow:   hostWindow,
	}, nil
}

func (l *RedisLimiter) Close() error {
	return l.client.Close()
}

func (l *RedisLimiter) CheckGlobal(ctx context.Context, fingerprint string) error {
	key := "rate:" + fingerprint
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if count == 1 {
		if err := l.client.Expire(ctx, key, l.globalWindow).Err(); err != nil {
			return err
		}
	}
	if count > int64(l.globalLimit) {
		return ErrRateLimited
	}
	return nil
}

func (l *RedisLimiter) ReserveHost(ctx context.Context, fingerprint string, host string) (func(context.Context), error) {
	key := fmt.Sprintf("report:%s:%s", fingerprint, host)
	ok, err := l.client.SetNX(ctx, key, "1", l.hostWindow).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrRateLimited
	}
	return func(ctx context.Context) {
		_ = l.client.Del(ctx, key).Err()
	}, nil
}
