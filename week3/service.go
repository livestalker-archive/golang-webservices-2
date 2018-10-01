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

// Microservice
type MyMicroservice struct {
	ctx        context.Context
	ListenAddr string
	ACL        map[string][]string
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
	return nil
}

func (b *AdminLogic) Statistics(in *StatInterval, s Admin_StatisticsServer) error {
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
	if err != nil {
		return nil, err
	}
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
	if err != nil {
		return err
	}
	err = handler(srv, ss)
	return err
}
