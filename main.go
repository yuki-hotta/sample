package main

import (
	"context"
	"fmt"
	"time"

	"io"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"go.etcd.io/etcd/client/v3"
)

type withGoroutineID struct {
	out io.Writer
}

func (w withGoroutineID) Write(p []byte) (int, error) {
	// goroutine <id> [running]:
	firstline := []byte(strings.SplitN(string(debug.Stack()), "\n", 2)[0])
	return w.out.Write(append(firstline[:len(firstline)-10], p...))
}

type Sync struct {
	client *clientv3.Client
	done chan struct{}
}

type Event struct{
	Id int
	Err error
}

type Response struct {
	Events []*Event
}

func (r *Response) Err() error {
	return nil
}

type Request struct {
	doneCh chan struct{}
}

func (s *Sync) clientWatch(ctx context.Context) <-chan Response {
	watchChan := make(chan Response, 1)
	go func() {
		childCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		// for {
			e1 := &Event{Id: 1}
			e2 :=  &Event{Id: 2}
			select{
			case <-childCtx.Done():
				break
			case watchChan <- Response{Events: []*Event{e1, e2}}:
			}
		// }
	}()
	return watchChan
}

// gohan/syncer/etcdv3/etcd.go
func (s *Sync) Watch(ctx context.Context) <-chan *Event {
	responseChan := make(chan *Event, 32)

	go func() {
		defer close(responseChan)
		childCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		rch := s.clientWatch(childCtx)
		err := func() error {
			for {
				select {
				case <-childCtx.Done():
					return nil
				case wresp, ok := <-rch:
					if !ok {
						break
					}
					err := wresp.Err()
					if err != nil {
						return err
					}
					log.Println(wresp)
					for _, ev := range wresp.Events {
						select{
						case <-childCtx.Done():
							break
						case responseChan <- ev:
						}
					}
				}
			}
		}()
		if err != nil {
			select {
			case responseChan <- &Event{Err: err}:
			default:
			}
		}
		return
	}()

	return responseChan
}

type ISync struct {
	raw Sync
}

type IEvent struct {
	Id int
}

// gohan/extension/goplugin/sync.go
func (syn *ISync) Watch(ctx context.Context, timeout time.Duration) ([]*IEvent, error) {
	eventChan := syn.raw.Watch(ctx)
	select {
	case event, ok := <-eventChan:
		if event == nil || !ok {
			return nil, nil
		}
		return []*IEvent{{
			Id: event.Id,
		}}, event.Err
	case <-time.After(timeout):
		return nil, nil
	case <-ctx.Done():
		return nil, context.Canceled
	}
}

type IEnv struct {
	sync *ISync
}

// esi_wan_gohan/northbound/common/longpoll.go
func watchOnce(ctx context.Context, env IEnv, timeout time.Duration) ([]*IEvent, error) {
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	return env.sync.Watch(childCtx, timeout)
}

// esi_wan_gohan/northbound/common/longpoll.go
func LongPoll(ctx context.Context, env IEnv, timeout time.Duration) (bool, error) {
	end := time.Now().Add(timeout)

	for {
		now := time.Now()
		if now.After(end) {
			break
		}

		events, err := watchOnce(ctx, env, timeout)

		if err == context.Canceled {
			break
		}
		if err != nil {
			return false, err
		}
		if events == nil {
			break
		}
		for _, event := range events {
			log.Println("get event : " + fmt.Sprintf("%+v", event) + ", num of goroutine = " + fmt.Sprintf("%d", runtime.NumGoroutine()))
		}
	}
	return false, nil
}

func main() {
	runtime.SetBlockProfileRate(1)
	log.SetOutput(withGoroutineID{out: os.Stderr})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := Sync{
		client: clientv3.NewCtxClient(ctx),
	}
	defer s.client.Close()

	env := IEnv {
		sync: &ISync{raw: s},
	}

	ok, err := LongPoll(ctx, env, time.Second * 10)
	log.Println(ok, err)
}
