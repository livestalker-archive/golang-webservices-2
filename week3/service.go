package main

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"log"
	"net"
	"regexp"
	"sync"
	"time"
)

type MyMicroservice struct {
	ctx         context.Context
	ListenAddr  string
	ACL         string
	ACLMap      map[string][]string
	LogChan     chan *Event
	LogWorkers  map[int]*LogWorker
	LWC         int
	LWCM        *sync.Mutex
	StatWorkers map[int]*StatWorker
	SWC         int
	SWCM        *sync.Mutex
	StatM       *sync.Mutex
	CurrentStat map[int]*Stat
}

type LogWorker struct {
	Cancel context.CancelFunc
	C      chan *Event
}

type StatWorker struct {
	Cancel context.CancelFunc
	//C      chan *Stat
}

// business logic
type BizLogic struct {
	svc *MyMicroservice
}

func NewBizLogic(svc *MyMicroservice) *BizLogic {
	return &BizLogic{svc: svc}
}

func (b *BizLogic) Check(ctx context.Context, in *Nothing) (*Nothing, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	p, _ := peer.FromContext(ctx)
	e := &Event{
		Consumer:  md.Get("consumer")[0],
		Method:    "/main.Biz/Check",
		Host:      p.Addr.String(),
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}
	b.svc.LogChan <- e
	return &Nothing{Dummy: true}, nil
}

func (b *BizLogic) Add(ctx context.Context, in *Nothing) (*Nothing, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	p, _ := peer.FromContext(ctx)
	e := &Event{
		Consumer:  md.Get("consumer")[0],
		Method:    "/main.Biz/Add",
		Host:      p.Addr.String(),
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}
	b.svc.LogChan <- e
	return &Nothing{Dummy: true}, nil
}

func (b *BizLogic) Test(ctx context.Context, in *Nothing) (*Nothing, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	p, _ := peer.FromContext(ctx)
	e := &Event{
		Consumer:  md.Get("consumer")[0],
		Method:    "/main.Biz/Test",
		Host:      p.Addr.String(),
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}
	b.svc.LogChan <- e
	return &Nothing{Dummy: true}, nil
}

type AdminLogic struct {
	svc *MyMicroservice
}

func (b *AdminLogic) Logging(in *Nothing, s Admin_LoggingServer) error {
	md, _ := metadata.FromIncomingContext(s.Context())
	p, _ := peer.FromContext(s.Context())
	e := &Event{
		Consumer:  md.Get("consumer")[0],
		Method:    "/main.Admin/Logging",
		Host:      p.Addr.String(),
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}
	b.svc.LogChan <- e
	time.Sleep(time.Microsecond * 1)
	b.svc.LWCM.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	worker := &LogWorker{Cancel: cancel, C: make(chan *Event, 1)}
	b.svc.LogWorkers[b.svc.LWC] = worker
	id := b.svc.LWC
	b.svc.LWC++
	b.svc.LWCM.Unlock()
	for {
		select {
		case <-ctx.Done():
			delete(b.svc.LogWorkers, id)
			log.Println("Finish logger: ", id)
			return nil
		case e := <-worker.C:
			s.Send(e)
		}
	}
	return nil
}

func (b *AdminLogic) Statistics(in *StatInterval, s Admin_StatisticsServer) error {
	sec := in.IntervalSeconds
	b.svc.SWCM.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	worker := &StatWorker{Cancel: cancel}
	b.svc.StatWorkers[b.svc.SWC] = worker
	id := b.svc.SWC
	b.svc.CurrentStat[b.svc.SWC] = &Stat{
		ByMethod:   make(map[string]uint64),
		ByConsumer: make(map[string]uint64),
	}
	b.svc.SWC++
	b.svc.SWCM.Unlock()
	for {
		t := time.NewTicker(time.Duration(sec) * time.Second)
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			b.svc.CurrentStat[id].Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
			s.Send(b.svc.CurrentStat[id])
		}
	}
	return nil
}

func NewAdminLogic(svc *MyMicroservice) *AdminLogic {
	return &AdminLogic{svc: svc}
}

func (svc *MyMicroservice) Start() error {
	// unpack acl
	err := svc.ParseACL()
	if err != nil {
		return err
	}
	// start listener
	lis, err := net.Listen("tcp", svc.ListenAddr)
	if err != nil {
		log.Fatalln("cant listen port", err)
	}
	server := grpc.NewServer(grpc.UnaryInterceptor(svc.ACLMidleware), grpc.StreamInterceptor(svc.ACLStreamMidleware))
	RegisterBizServer(server, NewBizLogic(svc))
	RegisterAdminServer(server, NewAdminLogic(svc))
	fmt.Println("starting server at ", svc.ListenAddr)
	logCtx, cancel := context.WithCancel(context.Background())
	logGroup := &sync.WaitGroup{}
	go server.Serve(lis)
	go func() {
		select {
		case <-svc.ctx.Done():
			cancel()
			logGroup.Wait()
			server.GracefulStop()
			return
		}
	}()
	// Logging supervisor
	logGroup.Add(1)
	go func() {
		defer logGroup.Done()
		for {
			select {
			case <-logCtx.Done():
				svc.LWCM.Lock()
				for _, v := range svc.LogWorkers {
					v.Cancel()
				}
				svc.LWCM.Unlock()
				return
			case e := <-svc.LogChan:
				svc.LWCM.Lock()
				for _, v := range svc.LogWorkers {
					v.C <- e
				}
				svc.LWCM.Unlock()
			}
		}
	}()
	return nil
}

func (svc *MyMicroservice) ParseACL() error {
	svc.ACLMap = make(map[string][]string)
	return json.Unmarshal([]byte(svc.ACL), &svc.ACLMap)
}

func (svc *MyMicroservice) ACLMidleware(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	acl := md.Get("consumer")
	if len(acl) != 0 {
		svc.incStatByConsumer(acl[0])
	}
	svc.incStatByMethod(info.FullMethod)
	if len(acl) == 0 {
		return &Nothing{Dummy: true}, grpc.Errorf(codes.Unauthenticated, "There are not ACL field")
	} else {
		if v, ok := svc.ACLMap[acl[0]]; !ok {
			return &Nothing{Dummy: true}, grpc.Errorf(codes.Unauthenticated, "Unknown consumer")
		} else {
			var allow bool
			for _, rule := range v {
				r := regexp.MustCompile(rule)
				if r.MatchString(info.FullMethod) {
					allow = true
					break
				}
			}
			if !allow {
				return &Nothing{Dummy: true}, grpc.Errorf(codes.Unauthenticated, "Unauthenticated")
			}
		}
	}
	reply, err := handler(ctx, req)
	return reply, err
}

func (svc *MyMicroservice) ACLStreamMidleware(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {
	md, _ := metadata.FromIncomingContext(ss.Context())
	acl := md.Get("consumer")
	if len(acl) == 0 {
		return grpc.Errorf(codes.Unauthenticated, "There are not ACL field")
	} else {
		if v, ok := svc.ACLMap[acl[0]]; !ok {
			return grpc.Errorf(codes.Unauthenticated, "Unknown consumer")
		} else {
			var allow bool
			for _, rule := range v {
				r := regexp.MustCompile(rule)
				if r.MatchString(info.FullMethod) {
					allow = true
					break
				}
			}
			if !allow {
				return grpc.Errorf(codes.Unauthenticated, "Unauthenticated")
			}
		}
	}
	svc.SWCM.Lock()
	if svc.SWC != 0 {
		if len(acl) != 0 {
			svc.incStatByConsumer(acl[0])
		}
		svc.incStatByMethod(info.FullMethod)
	}
	svc.SWCM.Unlock()
	err := handler(srv, ss)
	return err
}

func StartMyMicroservice(ctx context.Context, listenAddr string, acl string) error {
	ch := make(chan *Event)
	svc := &MyMicroservice{
		ctx:         ctx,
		ListenAddr:  listenAddr,
		ACL:         acl,
		LogChan:     ch,
		LogWorkers:  make(map[int]*LogWorker),
		LWCM:        &sync.Mutex{},
		StatWorkers: make(map[int]*StatWorker),
		SWCM:        &sync.Mutex{},
		StatM:       &sync.Mutex{},
		CurrentStat: make(map[int]*Stat),
		//CurrentStat: &Stat{
		//	ByMethod:   make(map[string]uint64),
		//	ByConsumer: make(map[string]uint64),
		//},
	}
	err := svc.Start()
	return err
}

func (svc *MyMicroservice) incStatByMethod(st *Stat, m string) {
	svc.StatM.Lock()
	if v, ok := st.ByMethod[m]; ok {
		st.ByMethod[m] = v + 1
	} else {
		st.ByMethod[m] = 1
	}
	svc.StatM.Unlock()
}

func (svc *MyMicroservice) incStatByConsumer(st *Stat, m string) {
	svc.StatM.Lock()
	if v, ok := st.ByConsumer[m]; ok {
		st.ByConsumer[m] = v + 1
	} else {
		st.ByConsumer[m] = 1
	}
	svc.StatM.Unlock()
}

func (svc *MyMicroservice) IncStat() {

}
