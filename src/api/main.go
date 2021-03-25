package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	pb "github.com/kenanwarren/k8s-playground/src/jobgen/proto"
	"google.golang.org/grpc"
)

var (
	wait       = flag.Duration("graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	port       = flag.Int("port", 10001, "The api server port")
	jobGenPort = flag.Int("jobGenPort", 10000, "The job gen server port")
	upgrader   = websocket.Upgrader{}
)

func main() {
	flag.Parse()

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", *jobGenPort), opts...)
	if err != nil {
		panic("Could not dial grpc connection")
	}
	defer conn.Close()
	client := pb.NewJobGenClient(conn)

	r := mux.NewRouter()
	r.HandleFunc("/jobs/crawl", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		reply, err := client.GenerateCrawlerJob(ctx, &pb.CrawlerJobRequest{Url: "http://example.org"})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %s\n", err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%s", reply.GetMessage())
	})
	r.HandleFunc("/jobs/chatter", func(w http.ResponseWriter, r *http.Request) {
		chatterTemplate.Execute(w, "ws://"+r.Host+"/jobs/chatter/ws")
	})
	r.HandleFunc("/jobs/chatter/ws", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		stream, err := client.StreamChatter(ctx, &pb.ChatterRequest{Name: "Test"})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %s\n", err.Error())
			return
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		defer c.Close()
		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("recv: %s", msg)

			chatter, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("%v.StreamChatter(_) = _, %v", client, err)
			}
			err = c.WriteMessage(mt, []byte(chatter.GetChatter()))
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	})
	srv := &http.Server{
		Addr:         fmt.Sprintf("localhost:%d", *port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
	}

	go func() {
		fmt.Printf("Server running on port %d\n", *port)
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	ctx, cancel := context.WithTimeout(context.Background(), *wait)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("shutting down")
	os.Exit(0)
}

var chatterTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>  
window.addEventListener("load", function(evt) {
    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;
    var print = function(message) {
        var d = document.createElement("div");
        d.textContent = message;
        output.appendChild(d);
    };
    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };
    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };
    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };
});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server, 
"Send" to send a message to the server and "Close" to close the connection. 
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value="Hello world!">
<button id="send">Send</button>
</form>
</td><td valign="top" width="50%">
<div id="output"></div>
</td></tr></table>
</body>
</html>
`))
