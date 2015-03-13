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

type RunRequest struct {
	DeployId string
}
type RunReply struct {
}

func (s *RpcServer) Run(arg RunRequest, reply *RunReply) error {
	return s.server.Run(arg.DeployId)
}

////////////////

type NewDeployDirRequest struct {
}
type NewDeployDirResponse struct {
	DeployId string
	Path     string
}

func (s *RpcServer) NewDeployDir(
	arg NewDeployDirRequest, reply *NewDeployDirResponse) error {

	resp := s.server.NewDeployDir()
	reply.Path = resp.Path
	reply.DeployId = resp.DeployId
	return nil
}
