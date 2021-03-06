package commands

import (
	"fmt"

	"github.com/abiosoft/ishell"
	"github.com/spf13/cobra"

	"github.com/qlcchain/go-qlc/common/types"
)

func addTxCmd() {
	if interactive {
		txCmd := &ishell.Cmd{
			Name: "tx",
			Help: "tx commands",
			Func: func(c *ishell.Context) {
				c.Println(c.Cmd.HelpText())
			},
		}
		shell.AddCmd(txCmd)

		addTxBlockInfoCmdByShell(txCmd)
		addTxBlockListCmdByShell(txCmd)
		addTxPendingCmdByShell(txCmd)
		addTxChangeCmdByShell(txCmd)
		addTxRecvCmdByShell(txCmd)
		addTxSendCmdByShell(txCmd)
		addTxRollbackCmdByShell(txCmd)
		addTxBatchSendByShell(txCmd)
		addSendToCreateByShell(txCmd)
	} else {
		var txCmd = &cobra.Command{
			Use:   "tx",
			Short: "tx commands",
			Run: func(cmd *cobra.Command, args []string) {
			},
		}
		rootCmd.AddCommand(txCmd)

		addTxPendingCmdByCobra(txCmd)
		addTxChangeCmdByCobra(txCmd)
		addTxRecvCmdByCobra(txCmd)
		addTxSendCmdByCobra(txCmd)
		addTxRollbackCmdByCobra(txCmd)
		addTxBatchSendByCobra(txCmd)
		addSendToCreateByCobra(txCmd)
	}
}

func txFormatBalance(amount types.Balance) string {
	n := float64(amount.Uint64())
	if n >= 1e8 {
		return fmt.Sprintf("%.2f", n/1e8)
	}
	return fmt.Sprintf("%.8f", n/1e8)
}
