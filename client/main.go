package main

import (
	"context"
	"flag"
	"fmt"
	chat_server "grpc-chat/proto"
	"io"
	"log"
	"os"
	"time"

	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/keyboard"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgets/text"
	"github.com/mum4k/termdash/widgets/textinput"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var (
	addr     string
	user     string
	pwd      string
	request  = make(chan string)
	response = make(chan *chat_server.ChatResponse)
)

func main() {
	flag.StringVar(&addr, "a", "localhost:8888", "server address:port")
	flag.StringVar(&user, "u", "", "user to login")
	flag.StringVar(&pwd, "p", "", "password of user")
	flag.Parse()

	if user == "" || pwd == "" {
		flag.Usage()
		os.Exit(1)
	}

	waitc := make(chan struct{})

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := chat_server.NewChatServerClient(conn)
	md := metadata.New(map[string]string{"user": user, "pwd": pwd})
	stream, err := client.Chat(metadata.NewOutgoingContext(context.Background(), md))
	if err != nil {
		log.Fatalf("fail to chat: %v", err)
	}

	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("chat failed: %v", err)
			}
			response <- in
		}
	}()

	go func() {
		for {
			stream.Send(&chat_server.ChatRequest{Msg: <-request})
		}
	}()

	go func() {
		t, err := tcell.New()
		if err != nil {
			panic(err)
		}
		defer t.Close()

		ctx, cancel := context.WithCancel(context.Background())
		input, err := textinput.New(
			textinput.Label(fmt.Sprintf(" %s : ", user), cell.FgColor(cell.ColorNumber(33))),
			textinput.MaxWidthCells(1000),
			textinput.Border(linestyle.Light),
			textinput.PlaceHolder("输入消息, 回车发送"),
		)
		if err != nil {
			panic(err)
		}
		rolled, err := text.New(text.RollContent(), text.WrapAtWords())
		if err != nil {
			panic(err)
		}

		c, err := container.New(
			t,
			container.Border(linestyle.Light),
			container.SplitHorizontal(
				container.Top(
					container.Border(linestyle.Light),
					container.BorderTitle("聊天消息"),
					container.PlaceWidget(rolled),
				),
				container.Bottom(
					container.Border(linestyle.Light),
					container.PlaceWidget(input),
					container.Focused(),
				),
				container.SplitPercent(80),
			),
		)
		if err != nil {
			panic(err)
		}

		quitter := func(k *terminalapi.Keyboard) {
			if k.Key == keyboard.KeyCtrlC {
				cancel()
				close(waitc)
			} else if k.Key == keyboard.KeyEnter {
				s := input.ReadAndClear()
				request <- s
			}
		}

		go func() {
			for {
				r := <-response
				rolled.Write(time.Now().Format("2006/01/02 15:04:05"), text.WriteCellOpts(cell.FgColor(cell.ColorYellow)))
				rolled.Write(fmt.Sprintf(" [%s] : ", r.User), text.WriteCellOpts(cell.FgColor(cell.ColorGreen)))
				rolled.Write(r.Msg + "\n")
			}
		}()

		if err := termdash.Run(ctx, t, c, termdash.KeyboardSubscriber(quitter)); err != nil {
			panic(err)
		}
	}()

	<-waitc
}
