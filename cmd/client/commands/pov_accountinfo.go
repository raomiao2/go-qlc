package commands

import (
	"fmt"

	"github.com/abiosoft/ishell"
	rpc "github.com/qlcchain/jsonrpc2"

	"github.com/qlcchain/go-qlc/cmd/util"
	cutil "github.com/qlcchain/go-qlc/common/util"
	"github.com/qlcchain/go-qlc/rpc/api"
)

func addPovAccountInfoCmdByShell(parentCmd *ishell.Cmd) {
	accountAddrFlag := util.Flag{
		Name:  "address",
		Must:  true,
		Usage: "address of account",
		Value: "",
	}
	args := []util.Flag{accountAddrFlag}
	cmd := &ishell.Cmd{
		Name:                "getAccountInfo",
		Help:                "get account state info",
		CompleterWithPrefix: util.OptsCompleter(args),
		Func: func(c *ishell.Context) {
			if util.HelpText(c, args) {
				return
			}
			if err := util.CheckArgs(c, args); err != nil {
				util.Warn(err)
				return
			}

			accountAddrStr := util.StringVar(c.Args, accountAddrFlag)

			err := runPovAccountInfoCmd(accountAddrStr)
			if err != nil {
				c.Println(err)
				return
			}
		},
	}
	parentCmd.AddCmd(cmd)
}

func runPovAccountInfoCmd(accountAddrStr string) error {
	client, err := rpc.Dial(endpointP)
	if err != nil {
		return err
	}
	defer client.Close()

	rspInfo := new(api.PovApiState)
	err = client.Call(rspInfo, "pov_getLatestAccountState", accountAddrStr)
	if err != nil {
		return err
	}

	fmt.Println(cutil.ToIndentString(rspInfo))

	return nil
}
