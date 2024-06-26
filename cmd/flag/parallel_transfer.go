package flag

import (
	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/spf13/cobra"
)

type ParallelTransferFlagValues struct {
	ThreadNumber       int
	TCPBufferSize      int
	tcpBufferSizeInput string
	RedirectToResource bool
	Icat               bool
}

var (
	parallelTransferFlagValues ParallelTransferFlagValues
)

func SetParallelTransferFlags(command *cobra.Command) {
	command.Flags().IntVar(&parallelTransferFlagValues.ThreadNumber, "thread_num", commons.TransferTreadNumDefault, "Specify the number of transfer threads")
	command.Flags().StringVar(&parallelTransferFlagValues.tcpBufferSizeInput, "tcp_buffer_size", commons.TcpBufferSizeStringDefault, "Specify TCP socket buffer size")
	command.Flags().BoolVar(&parallelTransferFlagValues.RedirectToResource, "redirect", false, "Always redirect to resource server")
	command.Flags().BoolVar(&parallelTransferFlagValues.Icat, "icat", false, "Always transfer data via iCAT")

	command.MarkFlagsMutuallyExclusive("redirect", "icat")
}

func GetParallelTransferFlagValues() *ParallelTransferFlagValues {
	size, _ := commons.ParseSize(parallelTransferFlagValues.tcpBufferSizeInput)
	parallelTransferFlagValues.TCPBufferSize = int(size)

	return &parallelTransferFlagValues
}
