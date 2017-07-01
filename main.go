package main

import (
	ref "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"flag"
	"google.golang.org/grpc"
	"log"
	"golang.org/x/net/context"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"bytes"
	"strings"
)

var (
	address = flag.String("address", "localhost:50051", "address of grpc server")
	cmd = flag.String("cmd", "ls", "command to execute")
)

func init() {
	flag.Parse()
}

func trimDotFunc(r rune) bool {
	return r == '.'
}

func dumpMethodAsProto(m *descriptor.MethodDescriptorProto) string {
	buf := &bytes.Buffer{}
	buf.WriteString("rpc ")
	buf.WriteString(*m.Name)
	buf.WriteByte('(')
	if m.ClientStreaming != nil && *m.ClientStreaming == true {
		buf.WriteString("stream ")
	}
	buf.WriteString(strings.TrimLeftFunc(*m.InputType, trimDotFunc))
	buf.WriteString(") returns (")
	if m.ServerStreaming != nil && *m.ServerStreaming == true {
		buf.WriteString("stream ")
	}
	buf.WriteString(strings.TrimLeftFunc(*m.OutputType, trimDotFunc))
	buf.WriteString("){")
	buf.WriteString("}")
	return buf.String()
}

func main() {
	conn, err := grpc.Dial(*address, grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	cl := ref.NewServerReflectionClient(conn)
	c := context.Background()
	stream, err := cl.ServerReflectionInfo(c)
	if err != nil {
		log.Fatal(err)
	}
	switch *cmd {
	case "ls":
		err = stream.Send(&ref.ServerReflectionRequest{
			Host: "localhost",
			MessageRequest: &ref.ServerReflectionRequest_ListServices{ListServices: ""},
		})
		if err != nil {
			log.Fatal(err)
		}
		resp, err := stream.Recv()
		if err != nil {
			log.Fatal(err)
		}
		for _, service := range resp.GetListServicesResponse().GetService() {
			err := stream.Send(&ref.ServerReflectionRequest{
				Host: "localhost",
				MessageRequest: &ref.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: service.Name},
			})
			if err != nil {
				log.Fatal(err)
			}
			fileSymbol, err := stream.Recv()
			if err != nil {
				log.Fatal(err)
			}
			for i, v := range fileSymbol.GetFileDescriptorResponse().FileDescriptorProto {
				log.Printf("hanling response %d\n", i)
				fd := new(descriptor.FileDescriptorProto)
				if err := proto.Unmarshal(v, fd); err != nil {
					log.Fatalf("Error at parsing file descriptor: %v\n", err)
				}
				log.Printf("%s\n", fd.String())
				log.Printf("Service %s has methods\n", service.Name)
				for _, srv := range fd.Service {
					for _, method := range srv.Method {
						log.Printf("\t%s", dumpMethodAsProto(method))
					}
				}
			}
			log.Printf("Service: %s\n", service.Name)
		}


	default:
		log.Fatalf("Unknown command: %s", *cmd)
	}

}
