package process

type statisticsProcessor struct {
	transactionsHandler TransactionsHandler
	accountsHandler     AccountsHandler
	stakeInfoHandler    StakeInfoHandler
}

func NewStatisticsProcessor(
	transactionsHandler TransactionsHandler,
	accountsHandler AccountsHandler,
	stakeInfoHandler StakeInfoHandler,
) (*statisticsProcessor, error) {
	return &statisticsProcessor{
		transactionsHandler: transactionsHandler,
		accountsHandler:     accountsHandler,
		stakeInfoHandler:    stakeInfoHandler,
	}, nil
}

func (sp *statisticsProcessor) ProcessAllAccounts(endEpoch uint32) ([]byte, error) {
	return sp.accountsHandler.ProcessAllAccounts(endEpoch)
}

func (sp *statisticsProcessor) ProcessAllTransactions(endEpoch uint32) ([]byte, error) {
	return sp.transactionsHandler.ProcessAllTxs(endEpoch)
}

func (sp *statisticsProcessor) ProcessStakeInfo(endEpoch uint32) ([]byte, error) {
	return sp.stakeInfoHandler.ProcessEpochs(endEpoch)
}
