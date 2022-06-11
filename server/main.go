package main

import (
	"fmt"
	chat_server "grpc-chat/proto"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"gopkg.in/ini.v1"
)

var Users map[string]string = make(map[string]string)

type Server struct {
	chat_server.UnimplementedChatServerServer
	clients map[string]chat_server.ChatServer_ChatServer
	mu      sync.Mutex
}

func (s *Server) Chat(stream chat_server.ChatServer_ChatServer) error {
	s.mu.Lock()
	md, _ := metadata.FromIncomingContext(stream.Context())
	user, pwd := md.Get("user")[0], md.Get("pwd")[0]
	s.clients[user] = stream
	s.mu.Unlock()
	if v, ok := Users[user]; !(ok && v == pwd) {
		return fmt.Errorf("login failed")
	}
	log.Printf("user 【%s】login\n", user)
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		log.Printf("received msg 【%s】from user 【%s】\n", in.Msg, user)
		for u, client := range s.clients {
			log.Printf("send msg from user 【%s】to user【%s】\n", user, u)
			err := client.Send(&chat_server.ChatResponse{Msg: in.Msg, User: user})
			if err != nil {
				log.Printf("send msg to user %s failed: %v", u, err)
				log.Printf("delete user %s session\n", u)
				s.mu.Lock()
				delete(s.clients, u)
				s.mu.Unlock()
			}
		}
	}
}

func main() {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		log.Fatalf("failed to load config.ini: %v", err)
	}

	for _, user := range cfg.Section("Auth").KeyStrings() {
		Users[user] = cfg.Section("Auth").Key(user).String()
	}

	addr := "localhost:" + cfg.Section("Server").Key("port").String()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("server listen at " + addr)
	log.Printf("load config users:\n\t" + strings.Join(cfg.Section("Auth").KeyStrings(), "\n\t"))
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	chat_server.RegisterChatServerServer(grpcServer, &Server{clients: make(map[string]chat_server.ChatServer_ChatServer)})
	grpcServer.Serve(l)
}
