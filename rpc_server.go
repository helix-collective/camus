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

type SetActivePortRequest struct {
	Port int
}
type SetActivePortReply struct {
}

func (s *RpcServer) SetActiveByPort(arg SetActivePortRequest,
	reply *SetActivePortReply) error {
	return s.server.SetActiveByPort(arg.Port)
}

////////////////

type SetActiveByIdRequest struct {
	Id string
}
type SetActiveByIdReply struct{}

func (s *RpcServer) SetActiveById(arg SetActiveByIdRequest,
	reply *SetActiveByIdReply) error {
	return s.server.SetActiveById(arg.Id)
}

////////////////

type DeployIdForCleanupRequest struct {
	Id string
}
type DeployIdForCleanupReply struct{}

func (s *RpcServer) CleanupDeploy(arg DeployIdForCleanupRequest,
	reply *DeployIdForCleanupReply) error {
	return s.server.CleanupDeploy(arg.Id)
}

////////////////

type RunRequest struct {
	DeployId string
}
type RunReply struct {
	Port int
}

func (s *RpcServer) Run(arg RunRequest, reply *RunReply) error {
	deployId, err := s.server.GetFullDeployIdFromShortName(arg.DeployId)
	if err != nil {
		return err
	}

	port, err := s.server.Run(deployId)
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

type GetDeploysPathRequest struct{}
type GetDeploysPathReply struct {
	Path string
}

func (s *RpcServer) GetDeploysPath(arg GetDeploysPathRequest, reply *GetDeploysPathReply) error {
	reply.Path = s.server.DeploysPath()
	return nil
}

////////////////

type StopDeployRequest struct {
	DeployId string
}
type StopDeployResponse struct {
}

func (s *RpcServer) StopDeploy(arg StopDeployRequest, reply *StopDeployResponse) error {
	deployId, err := s.server.GetFullDeployIdFromShortName(arg.DeployId)
	if err != nil {
		return err
	}

	return s.server.Stop(deployId)
}

////////////////

type KillUnknownProcessesRequest struct {
}

type KillUnknownProcessesResponse struct {
}

func (s *RpcServer) KillUnknownProcesses(arg KillUnknownProcessesRequest, reply *KillUnknownProcessesResponse) error {
	s.server.KillUnknownProcesses()
	return nil
}

////////////////

type ShutdownRequest struct {
}

type ShutdownResponse struct {
}

func (s *RpcServer) Shutdown(arg ShutdownRequest, reply *ShutdownResponse) error {
	s.server.Shutdown()
	return nil
}
