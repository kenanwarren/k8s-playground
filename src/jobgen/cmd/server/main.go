package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	pb "github.com/kenanwarren/k8s-playground/src/jobgen/proto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	tls      = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile = flag.String("cert_file", "", "The TLS cert file")
	keyFile  = flag.String("key_file", "", "The TLS key file")
	port     = flag.Int("port", 10000, "The server port")
	src      = rand.NewSource(time.Now().UnixNano())
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

type jobGenServer struct {
	pb.UnimplementedJobGenServer
	logger *zap.SugaredLogger
}

// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
func randString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			sb.WriteByte(letterBytes[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}

// StreamChatter generates fake text to simulate a one way info stream
func (s *jobGenServer) StreamChatter(cr *pb.ChatterRequest, stream pb.JobGen_StreamChatterServer) error {
	s.logger.Info("New chatter stream connection")
	respBase := pb.Chatter{Name: cr.Name}
	for {
		time.Sleep(time.Millisecond * 500)
		respBase.Chatter = randString(rand.Intn(256))
		if err := stream.Send(&respBase); err != nil {
			return err
		}
	}
}

func (s *jobGenServer) GenerateCrawlerJob(ctx context.Context, cjr *pb.CrawlerJobRequest) (*pb.CrawlerJobReply, error) {
	response, err := http.Get(cjr.Url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return &pb.CrawlerJobReply{Message: string(body)}, nil
}

func newServer(logger *zap.SugaredLogger) *jobGenServer {
	s := &jobGenServer{
		logger: logger,
	}
	return s
}

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err.Error())
	}
	defer logger.Sync() // flushes buffer, if any
	sugarLog := logger.Sugar()

	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		sugarLog.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if *tls {
		if *certFile == "" {
			panic("No provided tls cert")
		}
		if *keyFile == "" {
			panic("No provided tls key")
		}
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			sugarLog.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)

	pb.RegisterJobGenServer(grpcServer, newServer(sugarLog))
	fmt.Printf("Server running on port %d\n", *port)
	grpcServer.Serve(lis)
}
