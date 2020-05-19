/*
Copyright 2020 Betsson Group.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	oauth2v1 "github.com/BetssonGroup/dex-operator/api/v1"
	"github.com/BetssonGroup/dex-operator/controllers"
	dexapi "github.com/BetssonGroup/dex-operator/pkg/dex"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = oauth2v1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var dexGrpc string
	var dexClientCA string
	var dexClientCert string
	var dexClientKey string
	var healthAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&dexGrpc, "dex-grpc", "dex:35000", "Dex grpc host and port")
	flag.StringVar(&dexClientCA, "dex-grpc-ca", "/etc/dex/tls/ca.crt", "Path to the Dex GRPC CA")
	flag.StringVar(&dexClientCert, "dex-grpc-cert", "/etc/dex/tls/tls.crt", "Path to the Dex GRPC client certificate")
	flag.StringVar(&dexClientKey, "dex-grpc-key", "/etc/dex/tls/tls.key", "Path to the Dex GRPC client key")
	flag.StringVar(&healthAddr, "health-addr", ":9440", "The address the health endpoint binds to.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "02d43561.betssongroup.com",
		HealthProbeBindAddress: healthAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup a dex client
	dexOptions := &dexapi.Options{
		HostAndPort: dexGrpc,
		ClientCA:    dexClientCA,
		ClientCrt:   dexClientCert,
		ClientKey:   dexClientKey,
	}
	dexClient, err := dexapi.NewClient(dexOptions)
	if err != nil {
		setupLog.Error(err, "unable to setup Dex grcp client")
		os.Exit(1)
	}
	if err = (&controllers.ClientReconciler{
		Client:    mgr.GetClient(),
		Log:       ctrl.Log.WithName("controllers").WithName("Client"),
		Scheme:    mgr.GetScheme(),
		DexClient: dexClient,
		Recorder:  mgr.GetEventRecorderFor("dex-operator"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Client")
		os.Exit(1)
	}
	setupChecks(mgr)
	// +kubebuilder:scaffold:builder
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupChecks(mgr ctrl.Manager) {
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
}
