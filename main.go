package main

import (
	"flag"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	ref "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"log"
	//"github.com/golang/protobuf/proto"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"google.golang.org/grpc/credentials"
	"io/ioutil"
	"strings"
	"os"
)

var (
	address = flag.String("address", "localhost:50051", "address of grpc server")
	cmd     = flag.String("cmd", "ls", "command to execute: \n\tls - list services\n\tlsm - list of method in services\nfind_method - try to find method by it's name\n")

	useTls      = flag.Bool("use_tls", false, "use tls for server connection")
	tlsAuthType = flag.String("tls_auth_type", "", "tls auth type. Empty for default. One of [no_client_cert, request_client_cert, require_any_client_cert, verify_client_cert_if_given, require_and_verify_client")
	cert        = flag.String("cert", "", "cert file")
	key         = flag.String("key", "", "key file")
	ca          = flag.String("ca", "", "ca file")

	jsonify     = flag.Bool("json", false, "print data as json")
	l           = flag.Bool("l", false, "print more info if possible")
	meth        = flag.String("method", "", "method to find")
)

func init() {
	flag.Parse()
	log.SetFlags(0)
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

func authType() tls.ClientAuthType {
	switch *tlsAuthType {
	case "request_client_cert":
		return tls.RequestClientCert
	case "require_any_client_cert":
		return tls.RequireAndVerifyClientCert
	case "verify_client_cert_if_given":
		return tls.VerifyClientCertIfGiven
	case "require_and_verify_client_cert":
		return tls.RequireAndVerifyClientCert
	default:
		return tls.NoClientCert
	}
}

func tlsConfig() *tls.Config {
	cfg := &tls.Config{
		ClientAuth: authType(),
	}

	crt, err := tls.LoadX509KeyPair(*cert, *key)
	if err != nil {
		log.Fatalf("Error at loading x509 key pair: %v", err)
	}

	cfg.Certificates = append(cfg.Certificates, crt)

	if len(*ca) > 0 {
		caFile, err := ioutil.ReadFile(*ca)
		if err != nil {
			log.Fatalf("Can't read ca file: %v", err)
		}

		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caFile)
		cfg.RootCAs = pool
	}

	return cfg
}

type serverServices []string

func (ss serverServices) String() string {
	b := bytes.Buffer{}
	for _, v := range ss {
		b.WriteString(v)
		b.WriteByte('\n')
	}
	return b.String()
}

func (ss serverServices) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(ss))
}

func getServerServices(stream ref.ServerReflection_ServerReflectionInfoClient) (serverServices, error) {
	err := stream.Send(&ref.ServerReflectionRequest{
		Host:           "localhost",
		MessageRequest: &ref.ServerReflectionRequest_ListServices{ListServices: ""},
	})
	if err != nil {
		return nil, err
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	out := []string{}
	for _, service := range resp.GetListServicesResponse().Service {
		out = append(out, service.Name)
	}
	return out, nil
}

func getFileDescriptorProto(stream ref.ServerReflection_ServerReflectionInfoClient, service string) ([]*descriptor.FileDescriptorProto, error) {
	err := stream.Send(&ref.ServerReflectionRequest{
		Host:           "localhost",
		MessageRequest: &ref.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: service},
	})
	if err != nil {
		return nil, err
	}

	fileDescriptor, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	out := []*descriptor.FileDescriptorProto{}
	for _, v := range fileDescriptor.GetFileDescriptorResponse().FileDescriptorProto {
		fd := new(descriptor.FileDescriptorProto)
		if err := proto.Unmarshal(v, fd); err != nil {
			return nil, err
		}
		out = append(out, fd)
	}
	return out, nil
}

type serviceMethod struct {
	Service    string `json:"service"`
	Method     string `json:"method"`
	LongMethod string `json:"long_method,omitempty"`
}

type serviceMethods []serviceMethod

func serviceMethodsFromDescriptor(descs []*descriptor.FileDescriptorProto) serviceMethods {
	sm := serviceMethods{}
	for _, desc := range descs {
		for _, service := range desc.Service {
			for _, method := range service.Method {
				m := serviceMethod {
					Method: *method.Name,
					Service: *desc.Package + "." + *service.Name,
				}
				if *l {
					m.LongMethod = dumpMethodAsProto(method)
				}
				sm = append(sm, m)
			}
		}

	}
	return sm
}

func (sm serviceMethods) String() string {
	b := bytes.Buffer{}

	for i := range sm {
		b.WriteString(sm[i].Service)
		b.WriteByte('.')
		b.WriteString(sm[i].Method)
		b.WriteByte('\n')
		if len(sm[i].LongMethod) > 0 {
			b.WriteString(sm[i].LongMethod)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (sm serviceMethods) MarshalJSON() ([]byte, error) {
	return json.Marshal([]serviceMethod(sm))
}

func printResult(v interface{}) {
	if *jsonify {
		j, err := json.MarshalIndent(v, "", "\t")
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%s", j)
	} else {
		log.Printf("%s\n", v)
	}
}

func getServiceMethods(stream ref.ServerReflection_ServerReflectionInfoClient) (serviceMethods, error) {
	services, err := getServerServices(stream)
	if err != nil {
		return nil, err
	}
	dscrpts := []*descriptor.FileDescriptorProto{}
	for _, srv := range services {
		dscrpt, err := getFileDescriptorProto(stream, srv)
		if err != nil {
			return nil, err
		}
		dscrpts = append(dscrpts, dscrpt...)
	}


	return serviceMethodsFromDescriptor(dscrpts), nil
}

func main() {
	opts := []grpc.DialOption{}
	if *useTls {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig())))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	conn, err := grpc.Dial(*address, opts...)
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
		services, err := getServerServices(stream)
		if err != nil {
			log.Fatal(err)
		}
		printResult(services)
	case "lsm":
		sm, err := getServiceMethods(stream)
		if err != nil {
			log.Fatal(err)
		}

		printResult(sm)
	case "find_method":
		if len(*meth) == 0 {
			log.Fatalf("you must pass `method` argument")
		}

		sm, err := getServiceMethods(stream)
		if err != nil {
			log.Fatal(err)
		}

		found := false
		for _, s := range sm {
			if strings.Index(s.Method, *meth) != -1 {
				printResult(s)
				found = true
			}
		}
		if found {
			os.Exit(0)
		}
		os.Exit(1)
	default:
		log.Fatalf("Unknown command: %s", *cmd)
	}

}
