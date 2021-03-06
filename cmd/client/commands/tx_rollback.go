package commands

import (
	"errors"
	"fmt"

	"github.com/abiosoft/ishell"
	rpc "github.com/qlcchain/jsonrpc2"
	"github.com/spf13/cobra"

	"github.com/qlcchain/go-qlc/cmd/util"
	"github.com/qlcchain/go-qlc/common/types"
)

func addTxRollbackCmdByShell(parentCmd *ishell.Cmd) {
	hash := util.Flag{
		Name:  "hash",
		Must:  true,
		Usage: "rollback transaction hash",
		Value: "",
	}
	args := []util.Flag{hash}
	c := &ishell.Cmd{
		Name:                "rollback",
		Help:                "rollback transaction",
		CompleterWithPrefix: util.OptsCompleter(args),
		Func: func(c *ishell.Context) {
			if util.HelpText(c, args) {
				return
			}
			if err := util.CheckArgs(c, args); err != nil {
				util.Warn(err)
				return
			}

			hashP := util.StringVar(c.Args, hash)
			err := rollbackAction(hashP)
			if err != nil {
				util.Warn(err)
				return
			}
			util.Info("rollback transaction success!")
		},
	}
	parentCmd.AddCmd(c)
}

func addTxRollbackCmdByCobra(parentCmd *cobra.Command) {
	var hashP string

	var c = &cobra.Command{
		Use:   "rollback",
		Short: "rollback transaction",
		Run: func(cmd *cobra.Command, args []string) {
			err := rollbackAction(hashP)
			if err != nil {
				cmd.Println(err)
				return
			}
			fmt.Println("rollback transaction success!")
		},
	}

	c.Flags().StringVarP(&hashP, "hash", "h", "", "rollback transaction hash")
	parentCmd.AddCommand(c)
}

func rollbackAction(hashP string) error {
	if hashP == "" {
		return errors.New("transaction hash is null")
	}

	hash, err := types.NewHash(hashP)
	if err != nil {
		return errors.New("err transaction hash")
	}

	if err := rollbackTx(hash); err != nil {
		return err
	}
	return nil
}

func rollbackTx(hash types.Hash) error {
	client, err := rpc.Dial(endpointP)
	if err != nil {
		return err
	}
	defer client.Close()

	err = client.Call(nil, "debug_rollback", hash)
	if err != nil {
		return err
	}
	return nil
}
