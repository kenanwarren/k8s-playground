package main

import (
	"context"
	"flag"
	"io"
	"log"
	"time"

	pb "github.com/kenanwarren/k8s-playground/src/jobgen/proto"
	"google.golang.org/grpc"
)

var (
	serverAddr = flag.String("server_addr", "localhost:10000", "The server address in the format of host:port")
)

// printChatter stuff
func printChatter(client pb.JobGenClient, req *pb.ChatterRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.StreamChatter(ctx, req)
	if err != nil {
		log.Fatalf("%v.StreamChatter(_) = _, %v", client, err)
	}
	for {
		chatter, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v.StreamChatter(_) = _, %v", client, err)
		}
		log.Printf("Initiater: %s Chatter: %s\n", chatter.GetName(), chatter.GetChatter())
	}
}

func printCrawl(client pb.JobGenClient, req *pb.CrawlerJobRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.GenerateCrawlerJob(ctx, req)
	if err != nil {
		log.Fatalf("%v.printCrawl(_) = _, %v", client, err)
	}
	log.Printf("Crawl URL: %s Crawl body:\n%s", req.GetUrl(), resp.GetMessage())
}

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewJobGenClient(conn)
	printCrawl(client, &pb.CrawlerJobRequest{Url: "http://example.org"})
	printChatter(client, &pb.ChatterRequest{Name: "Test"})
}
