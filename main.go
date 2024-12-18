package main

import (
	"context"
	"fmt"
	"time"
	syn "sync"
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
	Key string
	Value string
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

// gohan/syncer/etcdv3/etcd.go
func (s *Sync) watch(ctx context.Context, responseChan chan *Event) error {

	ctx, cancel := context.WithCancel(ctx)
	errorsCh := make(chan error, 1)
	var wg syn.WaitGroup
	wg.Add(1)
	go func() {

		defer wg.Done()
		err := func() error {
			rch := s.client.Watch(ctx, "/message")

			for wresp := range rch {
				err := wresp.Err()
				if err != nil {
					return err
				}
				for _, ev := range wresp.Events {
					func() {
						for _, kv := range []*struct{ 
							Key string
							Value string
						}{{ Key: string(ev.Kv.Key), Value: string(ev.Kv.Value)} } {
							event_ := &Event{
								Key: string(kv.Key), 
								Value: string(kv.Value),
							}
							select {
							case <-ctx.Done():
								return
							case responseChan <- event_:
							}
						}
					}()
				}
			}
			return nil
		}()
		errorsCh <- err
	}()
	defer func() {
		cancel()
		wg.Wait()
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errorsCh:
		return err
	}
}

// Watch keep watch update under the path until context is canceled
func (s *Sync) Watch(ctx context.Context) <-chan *Event {
	eventCh := make(chan *Event, 32)
	watchDoneCh := make(chan error, 1)
	go func() {
		watchDoneCh <- s.watch(ctx, eventCh)
	}()
	go func() {
		defer close(eventCh)

		select {
		case <-ctx.Done():
			// don't return without ensuring Watch finished or we risk panic:
			// send on closed eventCh channel
			<-watchDoneCh
		case err := <-watchDoneCh:
			if err != nil {
				select {
				case eventCh <- &Event{Err: err}:
				default:
				}
			}
		}
	}()
	return eventCh
}

type ISync struct {
	raw Sync
}

type IEvent struct {
	Key string
	Value string
}

// gohan/extension/goplugin/sync.go
func (syn *ISync) Watch(ctx context.Context, timeout time.Duration) ([]*IEvent, error) {
	eventChan := syn.raw.Watch(ctx)
	select {
	case event, _ := <-eventChan:
		return []*IEvent{{
			Key: event.Key,
			Value: event.Value,
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
		fmt.Println(events, err, ctx, env, timeout)
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

	cli, err := clientv3.New(
		clientv3.Config{
			Endpoints: []string{"localhost:2379"},
		},
	)
	s := Sync{
		client: cli,
	}
	defer s.client.Close()

	env := IEnv {
		sync: &ISync{raw: s},
	}

	ok, err := LongPoll(ctx, env, time.Second * 40)
	log.Println(ok, err)
}
