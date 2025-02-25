// Copyright 2023 The CubeFS Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package cmd

import (
	"math"
	"strconv"

	"github.com/cubefs/cubefs/proto"
	"github.com/cubefs/cubefs/sdk/master"
	"github.com/cubefs/cubefs/sdk/meta"
	"github.com/spf13/cobra"
)

const (
	cmdQuotaUse           = "quota [COMMAND]"
	cmdQuotaShort         = "Manage cluster quota"
	cmdQuotaSetUse        = "set [volname] [fullpath]"
	cmdQuotaSetShort      = "set path quota"
	cmdQuotaListUse       = "list [volname]"
	cmdQuotaListShort     = "list volname all quota"
	cmdQuotaUpdateUse     = "update [volname] [fullpath]"
	cmdQuotaUpdateShort   = "update path quota"
	cmdQuotaDeleteUse     = "delete [volname] [quotaId]"
	cmdQUotaDeleteShort   = "delete path quota"
	cmdQuotaGetInodeUse   = "getInode [volname] [inode]"
	cmdQuotaGetInodeShort = "get inode quotaInfo"
	cmdQuotaListAllUse    = "listAll"
	cmdQuotaListAllShort  = "list all volname has quota"
)

const (
	cmdQuotaDefaultMaxFiles = math.MaxUint64
	cmdQuotaDefaultMaxBytes = math.MaxUint64
)

func newQuotaCmd(client *master.MasterClient) *cobra.Command {
	var cmd = &cobra.Command{
		Use:     cmdQuotaUse,
		Short:   cmdQuotaShort,
		Args:    cobra.MinimumNArgs(0),
		Aliases: []string{"quota"},
	}
	proto.InitBufferPool(32768)
	cmd.AddCommand(
		newQuotaListCmd(client),
		newQuotaSetCmd(client),
		newQuotaUpdateCmd(client),
		newQuotaDelete(client),
		newQuotaGetInode(client),
		newQuotaListAllCmd(client),
	)
	return cmd
}

func newQuotaSetCmd(client *master.MasterClient) *cobra.Command {
	var maxFiles uint64
	var maxBytes uint64

	var cmd = &cobra.Command{
		Use:   cmdQuotaSetUse,
		Short: cmdQuotaSetShort,
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			volName := args[0]
			fullPath := args[1]

			var metaConfig = &meta.MetaConfig{
				Volume:  volName,
				Masters: client.Nodes(),
			}
			metaWrapper, err := meta.NewMetaWrapper(metaConfig)
			if err != nil {
				stdout("NewMetaWrapper failed: %v\n", err)
				return
			}
			inodeId, err := metaWrapper.LookupPath(fullPath)
			if err != nil {
				stdout("get inode by fullPath %v fail %v\n", fullPath, err)
				return
			}

			mp := metaWrapper.GetPartitionByInodeId_ll(inodeId)
			if mp == nil {
				stdout("can not find mp by inodeId: %v\n", inodeId)
				return
			}
			if err = client.AdminAPI().SetQuota(volName, fullPath, inodeId, mp.PartitionID, maxFiles, maxBytes); err != nil {
				stdout("volName %v path %v quota set failed(%v)\n", volName, fullPath, err)
				return
			}
			stdout("setQuota: volName %v path %v inode %v maxFiles %v maxBytes %v success.\n",
				volName, fullPath, inodeId, maxFiles, maxBytes)
		},
	}
	cmd.Flags().Uint64Var(&maxFiles, CliFlagMaxFiles, cmdQuotaDefaultMaxFiles, "Specify quota max files")
	cmd.Flags().Uint64Var(&maxBytes, CliFlagMaxBytes, cmdQuotaDefaultMaxBytes, "Specify quota max bytes")
	return cmd
}

func newQuotaListCmd(client *master.MasterClient) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   cmdQuotaListUse,
		Short: cmdQuotaListShort,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var quotas []*proto.QuotaInfo
			var err error
			volName := args[0]
			if quotas, err = client.AdminAPI().ListQuota(volName); err != nil {
				stdout("volName %v quota list failed(%v)\n", volName, err)
				return
			}
			stdout("[quotas]\n")
			stdout("%v\n", formatQuotaTableHeader())
			for _, quotaInfo := range quotas {
				stdout("%v\n", formatQuotaInfo(quotaInfo))
			}
		},
	}
	return cmd
}

func newQuotaListAllCmd(client *master.MasterClient) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   cmdQuotaListAllUse,
		Short: cmdQuotaListAllShort,
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			var vols []*proto.VolInfo

			if vols, err = client.AdminAPI().ListQuotaAll(); err != nil {
				stdout("quota list all failed(%v)\n", err)
				return
			}
			stdout("%v\n", volumeInfoTableHeader)
			for _, vol := range vols {
				stdout("%v\n", formatVolInfoTableRow(vol))
			}
		},
	}

	return cmd
}

func newQuotaUpdateCmd(client *master.MasterClient) *cobra.Command {
	var maxFiles uint64
	var maxBytes uint64

	var cmd = &cobra.Command{
		Use:   cmdQuotaUpdateUse,
		Short: cmdQuotaUpdateShort,
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			volName := args[0]
			fullPath := args[1]

			var metaConfig = &meta.MetaConfig{
				Volume:  volName,
				Masters: client.Nodes(),
			}

			metaWrapper, err := meta.NewMetaWrapper(metaConfig)
			if err != nil {
				stdout("NewMetaWrapper failed: %v\n", err)
				return
			}
			inodeId, err := metaWrapper.LookupPath(fullPath)
			if err != nil {
				stdout("get inode by fullPath %v fail %v\n", fullPath, err)
				return
			}
			mp := metaWrapper.GetPartitionByInodeId_ll(inodeId)
			if mp == nil {
				stdout("can not find mp by inodeId: %v\n", inodeId)
				return
			}
			quotaInfo, err := client.AdminAPI().GetQuota(volName, fullPath)
			if err != nil {
				stdout("get quota vol: %v ,fullPath: %v failed err %v.\n", volName, fullPath, err)
				return
			}
			if maxFiles == 0 {
				maxFiles = quotaInfo.MaxFiles
			}
			if maxBytes == 0 {
				maxBytes = quotaInfo.MaxBytes
			}
			if err = client.AdminAPI().UpdateQuota(volName, fullPath, inodeId, mp.PartitionID, maxFiles, maxBytes); err != nil {
				stdout("volName %v path %v quota update failed(%v)\n", volName, fullPath, err)
				return
			}
			stdout("updateQuota: volName %v path %v inode %v maxFiles %v maxBytes %v success.\n",
				volName, fullPath, inodeId, maxFiles, maxBytes)
		},
	}
	cmd.Flags().Uint64Var(&maxFiles, CliFlagMaxFiles, 0, "Specify quota max files")
	cmd.Flags().Uint64Var(&maxBytes, CliFlagMaxBytes, 0, "Specify quota max bytes")
	return cmd
}

func newQuotaDelete(client *master.MasterClient) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   cmdQuotaDeleteUse,
		Short: cmdQUotaDeleteShort,
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			volName := args[0]
			fullPath := args[1]
			var err error
			if err = client.AdminAPI().DeleteQuota(volName, fullPath); err != nil {
				stdout("volName %v fullPath %v quota delete failed(%v)\n", volName, fullPath, err)
				return
			}
			stdout("deleteQuota: volName %v fullPath %v success.\n", volName, fullPath)
		},
	}
	return cmd
}

func newQuotaGetInode(client *master.MasterClient) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   cmdQuotaGetInodeUse,
		Short: cmdQuotaGetInodeShort,
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			volName := args[0]
			inodeId, err := strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				stdout("inodeId %v is illegal", args[2])
				return
			}

			var metaConfig = &meta.MetaConfig{
				Volume:  volName,
				Masters: client.Nodes(),
			}
			metaWrapper, err := meta.NewMetaWrapper(metaConfig)
			if err != nil {
				stdout("NewMetaWrapper failed: %v\n", err)
				return
			}

			quotaInfos, err := metaWrapper.GetInodeQuota_ll(inodeId)
			if err != nil {
				stdout("get indoe quota failed %v\n", err)
				return
			}
			for quotaId, quotaInfo := range quotaInfos {
				var status string
				if quotaInfo.Status == proto.QuotaInit {
					status = "Init"
				} else if quotaInfo.Status == proto.QuotaComplete {
					status = "Complete"
				} else {
					status = "Deleting"
				}
				stdout("quotaId [%v] rootInode [%v]  status [%v]\n", quotaId, quotaInfo.RootInode, status)
			}
		},
	}
	return cmd
}
