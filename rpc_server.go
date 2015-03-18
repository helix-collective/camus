package main

type RpcServer struct {
	server *ServerImpl
}

////////////////

type ListDeploysRequest struct{}
type ListDeploysReply struct {
	Deploys []*Deploy
}

func (s *RpcServer) ListDeploys(arg ListDeploysRequest, reply *ListDeploysReply) error {
	deploys, err := s.server.ListDeploys()
	if err != nil {
		return err
	}
	reply.Deploys = deploys
	return nil
}

////////////////

type SetMainPortRequest struct {
	Port int
}
type SetMainPortReply struct {
}

func (s *RpcServer) SetMainByPort(arg SetMainPortRequest,
	reply *RunReply) error {
	return s.server.SetMainByPort(arg.Port)
}

////////////////

type RunRequest struct {
	DeployId string
}
type RunReply struct {
	Port int
}

func (s *RpcServer) Run(arg RunRequest, reply *RunReply) error {
	port, err := s.server.Run(arg.DeployId)
	if err != nil {
		return err
	}

	reply.Port = port
	return nil
}

////////////////

type NewDeployDirRequest struct {
}
type NewDeployDirResponse struct {
	DeployId string
	Path     string
}

func (s *RpcServer) NewDeployDir(arg NewDeployDirRequest, reply *NewDeployDirResponse) error {
	resp := s.server.NewDeployDir()
	reply.Path = resp.Path
	reply.DeployId = resp.DeployId
	return nil
}

////////////////

type KillUnknownProcessesRequest struct {
}

type KillUnknownProcessesResponse struct {
	KilledPids []int
}

func (s *RpcServer) KillUnknownProcesses(arg KillUnknownProcessesRequest, reply *KillUnknownProcessesResponse) error {
	reply.KilledPids = s.server.KillUnknownProcesses()
	return nil
}
