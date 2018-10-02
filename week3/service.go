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

// Microservice
type MyMicroservice struct {
	ctx        context.Context
	ListenAddr string
	ACL        map[string][]string
	StatAgents *StatAgents
	LogAgents  *LogAgents
}

func (svc *MyMicroservice) parseAcl(textAcl string) error {
	svc.ACL = make(map[string][]string)
	return json.Unmarshal([]byte(textAcl), &svc.ACL)
}

func (svc *MyMicroservice) checkAcl(acl []string, method string) error {
	if len(acl) == 0 {
		return grpc.Errorf(codes.Unauthenticated, "There are not ACL field")
	} else {
		if v, ok := svc.ACL[acl[0]]; !ok {
			return grpc.Errorf(codes.Unauthenticated, "Unknown consumer")
		} else {
			var allow bool
			for _, rule := range v {
				r := regexp.MustCompile(rule)
				if r.MatchString(method) {
					allow = true
					break
				}
			}
			if !allow {
				return grpc.Errorf(codes.Unauthenticated, "Unauthenticated")
			}
		}
	}
	return nil
}

func StartMyMicroservice(ctx context.Context, listenAddr string, acl string) error {
	svc := &MyMicroservice{
		ctx:        ctx,
		ListenAddr: listenAddr,
		StatAgents: NewStatAgents(),
		LogAgents:  NewLogsAgents(),
	}
	// unpack acl
	err := svc.parseAcl(acl)
	if err != nil {
		return err
	}
	// start listener
	lis, err := net.Listen("tcp", svc.ListenAddr)
	if err != nil {
		log.Fatalln("cant listen port", err)
	}
	server := grpc.NewServer(grpc.UnaryInterceptor(svc.UnaryMiddleware), grpc.StreamInterceptor(svc.StreamMiddleware))
	RegisterBizServer(server, NewBizLogic(svc))
	RegisterAdminServer(server, NewAdminLogic(svc))
	fmt.Println("starting server at ", svc.ListenAddr)
	go server.Serve(lis)
	go func() {
		select {
		case <-svc.ctx.Done():
			server.GracefulStop()
			svc.StatAgents.StopAll()
			svc.LogAgents.StopAll()
			return
		}
	}()
	return nil
}

// Business logic
type BizLogic struct {
	svc *MyMicroservice
}

func NewBizLogic(svc *MyMicroservice) *BizLogic {
	return &BizLogic{svc: svc}
}

func (b *BizLogic) Check(ctx context.Context, in *Nothing) (*Nothing, error) {
	return &Nothing{Dummy: true}, nil
}

func (b *BizLogic) Add(ctx context.Context, in *Nothing) (*Nothing, error) {
	return &Nothing{Dummy: true}, nil
}

func (b *BizLogic) Test(ctx context.Context, in *Nothing) (*Nothing, error) {
	return &Nothing{Dummy: true}, nil
}

// Admin logic
type AdminLogic struct {
	svc *MyMicroservice
}

func NewAdminLogic(svc *MyMicroservice) *AdminLogic {
	return &AdminLogic{svc: svc}
}

func (b *AdminLogic) Logging(in *Nothing, s Admin_LoggingServer) error {
	la := b.svc.LogAgents.AllocateNew()
	for {
		select {
		case <-la.Ctx.Done():
			return nil
		case e := <-la.C:
			s.Send(e)
		}
	}
	return nil
}

func (b *AdminLogic) Statistics(in *StatInterval, s Admin_StatisticsServer) error {
	sec := in.IntervalSeconds
	sa := b.svc.StatAgents.AllocateNew()
	t := sa.SetTimer(sec)
	for {
		select {
		case <-sa.Ctx.Done():
			return nil
		case <-t.C:
			s.Send(sa.GetStat())
			sa.ResetStat()
		}
	}
	return nil
}

// Middleware
func (svc *MyMicroservice) UnaryMiddleware(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	acl := md.Get("consumer")
	err := svc.checkAcl(acl, info.FullMethod)
	svc.StatAgents.BroadcastIncByMethod(info.FullMethod)
	if len(acl) > 0 {
		svc.StatAgents.BroadcastIncByConsumer(acl[0])
	}
	if err != nil {
		return nil, err
	}
	p, _ := peer.FromContext(ctx)
	e := NewEvent(p.Addr.String(), acl[0], info.FullMethod)
	svc.LogAgents.BroadcastEvent(e)
	reply, err := handler(ctx, req)
	return reply, err
}

func (svc *MyMicroservice) StreamMiddleware(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {
	md, _ := metadata.FromIncomingContext(ss.Context())
	acl := md.Get("consumer")
	err := svc.checkAcl(acl, info.FullMethod)
	svc.StatAgents.BroadcastIncByMethod(info.FullMethod)
	if len(acl) > 0 {
		svc.StatAgents.BroadcastIncByConsumer(acl[0])
	}
	if err != nil {
		return err
	}
	p, _ := peer.FromContext(ss.Context())
	e := NewEvent(p.Addr.String(), acl[0], info.FullMethod)
	svc.LogAgents.BroadcastEvent(e)
	err = handler(srv, ss)
	return err
}

// Stat Agents
type StatAgents struct {
	list []*StatAgent
	sync.Mutex
}

func (sas *StatAgents) AllocateNew() *StatAgent {
	sas.Lock()
	sa := NewStatAgent()
	sas.list = append(sas.list, sa)
	sas.Unlock()
	return sa
}

func (sas *StatAgents) BroadcastIncByMethod(method string) {
	sas.Lock()
	for _, el := range sas.list {
		el.IncByMethod(method)
	}
	sas.Unlock()
}

func (sas *StatAgents) BroadcastIncByConsumer(method string) {
	sas.Lock()
	for _, el := range sas.list {
		el.IncByConsumer(method)
	}
	sas.Unlock()
}

func (sas *StatAgents) StopAll() {
	sas.Lock()
	for _, el := range sas.list {
		el.Cancel()
	}
	sas.Unlock()
}

type StatAgent struct {
	Stat   *Stat
	Ctx    context.Context
	Cancel context.CancelFunc
	sync.Mutex
}

func (sa *StatAgent) ResetStat() {
	sa.Lock()
	sa.Stat = NewStat()
	sa.Unlock()
}

func (sa *StatAgent) IncByMethod(method string) {
	sa.Lock()
	if v, ok := sa.Stat.ByMethod[method]; !ok {
		sa.Stat.ByMethod[method] = 1
	} else {
		sa.Stat.ByMethod[method] = v + 1
	}
	sa.Unlock()
}

func (sa *StatAgent) IncByConsumer(method string) {
	sa.Lock()
	if v, ok := sa.Stat.ByConsumer[method]; !ok {
		sa.Stat.ByConsumer[method] = 1
	} else {
		sa.Stat.ByConsumer[method] = v + 1
	}
	sa.Unlock()
}

func (sa *StatAgent) SetTimer(sec uint64) *time.Ticker {
	t := time.NewTicker(time.Duration(sec) * time.Second)
	return t
}

func (sa *StatAgent) GetStat() *Stat {
	sa.Lock()
	sa.Stat.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	sa.Unlock()
	return sa.Stat
}

func NewStat() *Stat {
	s := &Stat{
		ByMethod:   make(map[string]uint64),
		ByConsumer: make(map[string]uint64),
	}
	return s
}

func NewStatAgent() *StatAgent {
	ctx, cancel := context.WithCancel(context.Background())
	s := NewStat()
	sa := &StatAgent{Stat: s, Ctx: ctx, Cancel: cancel}
	return sa
}

func NewStatAgents() *StatAgents {
	sas := &StatAgents{
		list: make([]*StatAgent, 0),
	}
	return sas
}

// Log Agents
type LogAgents struct {
	list []*LogAgent
	sync.Mutex
}

func (las *LogAgents) AllocateNew() *LogAgent {
	las.Lock()
	la := NewLogAgent()
	las.list = append(las.list, la)
	las.Unlock()
	return la
}

func (las *LogAgents) BroadcastEvent(e *Event) {
	las.Lock()
	for _, el := range las.list {
		el.C <- e
	}
	las.Unlock()
}

func (las *LogAgents) StopAll() {
	las.Lock()
	for _, el := range las.list {
		el.Cancel()
	}
	las.Unlock()
}

type LogAgent struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	C      chan *Event
	sync.Mutex
}

func NewEvent(host string, consumer string, method string) *Event {
	e := &Event{
		Consumer:  consumer,
		Method:    method,
		Host:      host,
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}
	return e
}

func NewLogAgent() *LogAgent {
	ctx, cancel := context.WithCancel(context.Background())
	la := &LogAgent{Ctx: ctx, Cancel: cancel, C: make(chan *Event)}
	return la
}

func NewLogsAgents() *LogAgents {
	las := &LogAgents{
		list: make([]*LogAgent, 0),
	}
	return las
}
