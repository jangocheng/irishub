package clitest

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/tests"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/irisnet/irishub/app"
	//sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/irisnet/irishub/modules/gov"
)

func init() {
	irisHome, iriscliHome = getTestingHomeDirs()
}

func TestIrisCLISoftwareUpgrade(t *testing.T) {
	tests.ExecuteT(t, fmt.Sprintf("iris --home=%s unsafe_reset_all", irisHome), "")
	executeWrite(t, fmt.Sprintf("iriscli keys delete --home=%s foo", iriscliHome), app.DefaultKeyPass)
	executeWrite(t, fmt.Sprintf("iriscli keys delete --home=%s bar", iriscliHome), app.DefaultKeyPass)
	chainID := executeInit(t, fmt.Sprintf("iris init -o --name=foo --home=%s --home-client=%s", irisHome, iriscliHome))
	executeWrite(t, fmt.Sprintf("iriscli keys add --home=%s bar", iriscliHome), app.DefaultKeyPass)

	err := modifyGenesisFile(t, irisHome)
	require.NoError(t, err)

	// get a free port, also setup some common flags
	servAddr, port, err := server.FreeTCPAddr()
	require.NoError(t, err)
	flags := fmt.Sprintf("--home=%s --node=%v --chain-id=%v", iriscliHome, servAddr, chainID)

	// start iris server
	proc := tests.GoExecuteTWithStdout(t, fmt.Sprintf("iris start --home=%s --rpc.laddr=%v", irisHome, servAddr))

	defer proc.Stop(false)
	tests.WaitForTMStart(port)
	tests.WaitForNextNBlocksTM(2, port)

	fooAddr, _ := executeGetAddrPK(t, fmt.Sprintf("iriscli keys show foo --output=json --home=%s", iriscliHome))

	fooAcc := executeGetAccount(t, fmt.Sprintf("iriscli bank account %s %v", fooAddr, flags))
	fooCoin := convertToIrisBaseAccount(t, fooAcc)
	require.Equal(t, "100iris", fooCoin)

	// submit a upgrade proposal
	spStr := fmt.Sprintf("iriscli gov submit-proposal %v", flags)
	spStr += fmt.Sprintf(" --from=%s", "foo")
	spStr += fmt.Sprintf(" --deposit=%s", "10iris")
	spStr += fmt.Sprintf(" --type=%s", "SoftwareUpgrade")
	spStr += fmt.Sprintf(" --title=%s", "Upgrade")
	spStr += fmt.Sprintf(" --description=%s", "test")
	spStr += fmt.Sprintf(" --fee=%s", "0.004iris")

	executeWrite(t, spStr, app.DefaultKeyPass)
	tests.WaitForNextNBlocksTM(2, port)

	proposal1 := executeGetProposal(t, fmt.Sprintf("iriscli gov query-proposal --proposal-id=1 --output=json %v", flags))
	require.Equal(t, int64(1), proposal1.ProposalID)
	require.Equal(t, gov.StatusVotingPeriod, proposal1.Status)

	votingStartBlock1 := proposal1.VotingStartBlock

	voteStr := fmt.Sprintf("iriscli gov vote %v", flags)
	voteStr += fmt.Sprintf(" --from=%s", "foo")
	voteStr += fmt.Sprintf(" --proposal-id=%s", "1")
	voteStr += fmt.Sprintf(" --option=%s", "Yes")
	voteStr += fmt.Sprintf(" --fee=%s", "0.004iris")

	executeWrite(t, voteStr, app.DefaultKeyPass)
	tests.WaitForNextNBlocksTM(2, port)

	votes := executeGetVotes(t, fmt.Sprintf("iriscli gov query-votes --proposal-id=1 --output=json %v", flags))
	require.Len(t, votes, 1)
	require.Equal(t, int64(1), votes[0].ProposalID)
	require.Equal(t, gov.OptionYes, votes[0].Option)

	tests.WaitForHeightTM(votingStartBlock1+20, port)
	proposal1 = executeGetProposal(t, fmt.Sprintf("iriscli gov query-proposal --proposal-id=1 --output=json %v", flags))
	require.Equal(t, int64(1), proposal1.ProposalID)
	require.Equal(t, gov.StatusPassed, proposal1.Status)

	/////////////// Stop and Run new version Software ////////////////////
	// kill iris
	proc.Stop(true)

	// start iris1 server
	proc1 := tests.GoExecuteTWithStdout(t, fmt.Sprintf("iris1 start --home=%s --rpc.laddr=%v", irisHome, servAddr))
	defer proc1.Stop(false)

	// check the upgrade info
	//upgradeInfo := executeGetUpgradeInfo(t, fmt.Sprintf("iriscli1 upgrade info --output=json %v", flags))

}
