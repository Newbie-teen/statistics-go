package statistics

type StatsHandler interface {
	ProcessAllAccounts(endEpoch uint32) ([]byte, error)
	ProcessAllTransactions(endEpoch uint32) ([]byte, error)
	ProcessStakeInfo(endEpoch uint32) ([]byte, error)
}
