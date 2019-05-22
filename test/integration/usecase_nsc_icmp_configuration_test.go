// +build usecase

package nsmd_integration_tests

import (
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/networkservicemesh/networkservicemesh/test/kube_testing/pods"

	"github.com/networkservicemesh/networkservicemesh/test/integration/utils"
	"github.com/networkservicemesh/networkservicemesh/test/kube_testing"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

func TestNSCAndICMPLocal(t *testing.T) {
	RegisterTestingT(t)

	if testing.Short() {
		t.Skip("Skip, please run without -short")
		return
	}

	testNSCAndICMP(t, 1, false, false)
}

func TestNSCAndICMPRemote(t *testing.T) {
	RegisterTestingT(t)

	if testing.Short() {
		t.Skip("Skip, please run without -short")
		return
	}

	testNSCAndICMP(t, 2, false, false)
}

func TestNSCAndICMPWebhookLocal(t *testing.T) {
	RegisterTestingT(t)

	if testing.Short() {
		t.Skip("Skip, please run without -short")
		return
	}

	testNSCAndICMP(t, 1, true, false)
}

func TestNSCAndICMPWebhookRemote(t *testing.T) {
	RegisterTestingT(t)

	if testing.Short() {
		t.Skip("Skip, please run without -short")
		return
	}

	testNSCAndICMP(t, 2, true, false)
}

func TestNSCAndICMPLocalVeth(t *testing.T) {
	RegisterTestingT(t)

	if testing.Short() {
		t.Skip("Skip, please run without -short")
		return
	}

	testNSCAndICMP(t, 1, false, true)
}

func TestNSCAndICMPRemoteVeth(t *testing.T) {
	RegisterTestingT(t)

	if testing.Short() {
		t.Skip("Skip, please run without -short")
		return
	}

	testNSCAndICMP(t, 2, false, true)
}

func TestNSCAndICMPNeighbors(t *testing.T) {
	RegisterTestingT(t)

	if testing.Short() {
		t.Skip("Skip, please run without -short")
		return
	}

	k8s, err := kube_testing.NewK8s(true)
	defer k8s.Cleanup()

	Expect(err).To(BeNil())

	nodes_setup := utils.SetupNodes(k8s, 1, defaultTimeout)
	_ = utils.DeployNeighborNSE(k8s, nodes_setup[0].Node, "icmp-responder-nse-1", defaultTimeout)
	nsc := utils.DeployNSC(k8s, nodes_setup[0].Node, "nsc-1", defaultTimeout)

	pingResponse, errOut, err := k8s.Exec(nsc, nsc.Spec.Containers[0].Name, "ping", "172.16.1.2", "-A", "-c", "5")
	Expect(err).To(BeNil())
	Expect(errOut).To(Equal(""))
	Expect(strings.Contains(pingResponse, "100% packet loss")).To(Equal(false))

	nsc2 := utils.DeployNSC(k8s, nodes_setup[0].Node, "nsc-2", defaultTimeout)
	arpResponse, errOut, err := k8s.Exec(nsc2, nsc.Spec.Containers[0].Name, "arp", "-a")
	Expect(err).To(BeNil())
	Expect(errOut).To(Equal(""))
	Expect(strings.Contains(arpResponse, "172.16.1.2")).To(Equal(true))

}

/**
If passed 1 both will be on same node, if not on different.
*/
func testNSCAndICMP(t *testing.T, nodesCount int, useWebhook bool, disableVHost bool) {
	k8s, err := kube_testing.NewK8s(true)
	defer k8s.Cleanup()

	Expect(err).To(BeNil())

	if useWebhook {
		awc, awDeployment, awService := utils.DeployAdmissionWebhook(k8s, "nsm-admission-webhook", "networkservicemesh/admission-webhook", k8s.GetK8sNamespace())
		defer utils.DeleteAdmissionWebhook(k8s, "nsm-admission-webhook-certs", awc, awDeployment, awService, k8s.GetK8sNamespace())
	}

	config := []*pods.NSMgrPodConfig{}
	for i := 0; i < nodesCount; i++ {
		cfg := &pods.NSMgrPodConfig{
			Variables: pods.DefaultNSMD(),
		}
		cfg.Namespace = k8s.GetK8sNamespace()
		cfg.DataplaneVariables = utils.DefaultDataplaneVariables()
		if disableVHost {
			cfg.DataplaneVariables["DATAPLANE_ALLOW_VHOST"] = "false"
		}
		config = append(config, cfg)
	}
	nodes_setup := utils.SetupNodesConfig(k8s, nodesCount, defaultTimeout, config, k8s.GetK8sNamespace())

	// Run ICMP on latest node
	_ = utils.DeployICMP(k8s, nodes_setup[nodesCount-1].Node, "icmp-responder-nse-1", defaultTimeout)

	var nscPodNode *v1.Pod
	if useWebhook {
		nscPodNode = utils.DeployNSCWebhook(k8s, nodes_setup[0].Node, "nsc-1", defaultTimeout)
	} else {
		nscPodNode = utils.DeployNSC(k8s, nodes_setup[0].Node, "nsc-1", defaultTimeout)
	}
	var nscInfo *utils.NSCCheckInfo

	failures := InterceptGomegaFailures(func() {
		nscInfo = utils.CheckNSC(k8s, t, nscPodNode)
	})
	// Do dumping of container state to dig into what is happened.
	if len(failures) > 0 {
		logrus.Errorf("Failures: %v", failures)
		utils.PrintLogs(k8s, nodes_setup)
		nscInfo.PrintLogs()

		t.Fail()
	}
}
