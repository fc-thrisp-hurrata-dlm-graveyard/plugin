package plugin

import (
	"io"
	"log"
	"net/rpc"
	"os"
	"os/exec"
	"time"
)

type Plugin struct {
	name, path string
	*rpc.Server
	io.ReadWriteCloser
}

func (p *Plugin) Close() error {
	return p.ReadWriteCloser.Close()
}

func (p *Plugin) Serve() {
	p.Server.ServeConn(p)
}

func (p *Plugin) ServeCodec(fn func(io.ReadWriteCloser) rpc.ServerCodec) {
	p.Server.ServeCodec(fn(p))
}

func New(name, path string, api interface{}) *Plugin {
	p := &Plugin{
		name:            name,
		path:            path,
		Server:          rpc.NewServer(),
		ReadWriteCloser: rwc(os.Stdin, os.Stdout),
	}
	if err := p.RegisterName(name, api); err != nil {
		log.Fatalf("failed to register Plugin %s: %s", name, err)
	}
	return p
}

func Start(output io.Writer, path string, args ...string) (*rpc.Client, error) {
	pipe, err := start(makeCommand(output, path, args))
	if err != nil {
		return nil, err
	}
	return rpc.NewClient(pipe), nil
}

func StartCodec(
	fn func(io.ReadWriteCloser) rpc.ClientCodec,
	output io.Writer,
	path string,
	args ...string) (*rpc.Client, error) {
	pipe, err := start(makeCommand(output, path, args))
	if err != nil {
		return nil, err
	}
	return rpc.NewClientWithCodec(fn(pipe)), nil
}

var makeCommand = func(w io.Writer, path string, args []string) commander {
	cmd := exec.Command(path, args...)
	cmd.Stderr = w
	return execCmd{cmd}
}

//func StartConsumer(output io.Writer, path string, args ...string) (Server, error) {
//	pipe, err := start(makeCommand(output, path, args))
//	if err != nil {
//		return Server{}, err
//	}
//	return Server{
//		Server:          rpc.NewServer(),
//		ReadWriteCloser: pipe,
//	}, nil
//}

//func NewConsumer() *rpc.Client {
//	return rpc.NewClient(rwc(os.Stdin, os.Stdout))
//}

//func NewConsumerCodec(fn func(io.ReadWriteCloser) rpc.ClientCodec) *rpc.Client {
//	return rpc.NewClientWithCodec(fn(rwc(os.Stdin, os.Stdout)))
//}

type execCmd struct {
	*exec.Cmd
}

func (e execCmd) Start() (osProcess, error) {
	if err := e.Cmd.Start(); err != nil {
		return nil, err
	}
	return e.Cmd.Process, nil
}

type commander interface {
	StdinPipe() (io.WriteCloser, error)
	StdoutPipe() (io.ReadCloser, error)
	Start() (osProcess, error)
}

type osProcess interface {
	Wait() (*os.ProcessState, error)
	Kill() error
	Signal(os.Signal) error
}

type ioPipe struct {
	io.ReadCloser
	io.WriteCloser
	proc osProcess
}

func (iop ioPipe) Close() error {
	err := iop.ReadCloser.Close()
	if writeErr := iop.WriteCloser.Close(); writeErr != nil {
		err = writeErr
	}
	if procErr := iop.closeProc(); procErr != nil {
		err = procErr
	}
	return err
}

var (
	procTimeout          = time.Second
	ProcStopTimeoutError = Xrror("process killed after timeout waiting for process to stop")
	KillProcessError     = Xrror("error killing process after timeout: %s").Out
)

func (iop ioPipe) closeProc() error {
	result := make(chan error, 1)
	go func() { _, err := iop.proc.Wait(); result <- err }()
	if err := iop.proc.Signal(os.Interrupt); err != nil {
		return err
	}
	select {
	case err := <-result:
		return err
	case <-time.After(procTimeout):
		if err := iop.proc.Kill(); err != nil {
			return KillProcessError(err.Error())
		}
		return ProcStopTimeoutError
	}
}

func start(cmd commander) (ioPipe, error) {
	in, err := cmd.StdinPipe()
	if err != nil {
		return ioPipe{}, err
	}
	defer func() {
		if err != nil {
			in.Close()
		}
	}()
	out, err := cmd.StdoutPipe()
	if err != nil {
		return ioPipe{}, err
	}
	defer func() {
		if err != nil {
			out.Close()
		}
	}()

	proc, err := cmd.Start()
	if err != nil {
		return ioPipe{}, err
	}
	return ioPipe{out, in, proc}, nil
}

type rwCloser struct {
	io.ReadCloser
	io.WriteCloser
}

func rwc(r io.ReadCloser, w io.WriteCloser) rwCloser {
	return rwCloser{r, w}
}

func (r rwCloser) Close() error {
	var err error
	if err = r.ReadCloser.Close(); err != nil {
		return err
	}
	if err = r.WriteCloser.Close(); err != nil {
		return err
	}
	return nil
}
