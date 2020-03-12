package comm

type ClientInfo struct {
	WorkPath string
	ConfFile string
	Version  string
}

var G_CliInfo ClientInfo

type ReadFromServerConf struct {
	ServerAddress string
	CmdbAddress   string
	ECAddress     string
	CliTtl        int
	SoftCheck     string
}

var G_ReadFromServerConf *ReadFromServerConf

type ExecSque struct {
	//Ch_HttpServerDone       chan struct{}
	Ch_GetParaFormEtcdStart chan struct{}
	Ch_GetParaFormEtcdDone  chan struct{}
	Ch_CliRegStart          chan struct{}
	Ch_CliLogMonStart       chan struct{}
	Ch_ConnectCMStart       chan struct{}
	Ch_CheckFileStart       chan struct{}
	Ch_CheckFileDone        chan struct{}
}

var G_ExecSque *ExecSque
