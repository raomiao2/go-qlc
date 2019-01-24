/*
 * Copyright (c) 2019 QLC Chain Team
 *
 * This software is released under the MIT License.
 * https://opensource.org/licenses/MIT
 */

package commands

import (
	"encoding/hex"
	"fmt"
	"github.com/qlcchain/go-qlc/common/types"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	count      string
	seedString string
)

// sendCmd represents the send command
var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "generate account",
	Run: func(cmd *cobra.Command, args []string) {
		err := accountAction()
		if err != nil {
			cmd.Println(err)
		}
	},
}

func init() {
	accountCmd.Flags().StringVar(&count, "count", "10", "account count")
	accountCmd.Flags().StringVar(&seedString, "seed", "", "account seed")
	rootCmd.AddCommand(accountCmd)
}

func accountAction() error {
	if len(seedString) > 0 {
		bytes, err := hex.DecodeString(seedString)
		if err != nil {
			fmt.Println(err)
		}
		s, err := types.BytesToSeed(bytes)
		if err != nil {
			fmt.Println(err)
		}
		a, _ := s.Account(0)
		fmt.Printf("Seed: %s, Master Address: %s, PrivateKey: %s\n", s.String(),
			a.Address().String(), a.PrivateKey())
	} else {
		c, err := strconv.Atoi(count)
		if err != nil {
			fmt.Println(err)
		}

		for i := 0; i < c; i++ {
			seed, err := types.NewSeed()
			if err == nil {
				if a, err := seed.Account(0); err == nil {
					fmt.Printf("Seed: %s, Master Address: %s, PrivateKey: %s\n", seed.String(),
						a.Address().String(), a.PrivateKey())
				}
			}
		}
	}

	return nil
}