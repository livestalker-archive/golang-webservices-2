package main

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
	"regexp"
)

type MyMicroservice struct {
	ctx        context.Context
	ListenAddr string
	ACL        string
	ACLMap     map[string][]string
}

// business logic
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

type AdminLogic struct {
	svc *MyMicroservice
}

func (b *AdminLogic) Logging(in *Nothing, s Admin_LoggingServer) error {
	return nil
}

func (b *AdminLogic) Statistics(in *StatInterval, s Admin_StatisticsServer) error {
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
	go server.Serve(lis)
	go func() {
		select {
		case <-svc.ctx.Done():
			server.GracefulStop()
			return
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
	fmt.Println(md)
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
	err := handler(srv, ss)
	return err
}

func StartMyMicroservice(ctx context.Context, listenAddr string, acl string) error {
	svc := &MyMicroservice{ctx: ctx, ListenAddr: listenAddr, ACL: acl}
	err := svc.Start()
	return err
}
