package handlers

import (
	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	"context"
	"fmt"
	"github.com/Octops/gameserver-ingress-controller/internal/runtime"
	"github.com/Octops/gameserver-ingress-controller/pkg/gameserver"
	"github.com/Octops/gameserver-ingress-controller/pkg/k8sutil"
	"github.com/Octops/gameserver-ingress-controller/pkg/reconcilers"
	"github.com/Octops/gameserver-ingress-controller/pkg/record"
	"github.com/Octops/gameserver-ingress-controller/pkg/stores"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"time"
)

type GameSeverEventHandler struct {
	logger               *logrus.Entry
	client               *kubernetes.Clientset
	serviceReconciler    *reconcilers.ServiceReconciler
	ingressReconciler    *reconcilers.IngressReconciler
	gameserverReconciler *reconcilers.GameServerReconciler
}

func NewGameSeverEventHandler(store *stores.Store, agones *stores.AgonesStore, recorder *record.EventRecorder) *GameSeverEventHandler {
	return &GameSeverEventHandler{
		logger:               runtime.Logger().WithField("component", "event_handler"),
		serviceReconciler:    reconcilers.NewServiceReconciler(store, recorder),
		ingressReconciler:    reconcilers.NewIngressReconciler(store, recorder),
		gameserverReconciler: reconcilers.NewGameServerReconciler(agones, recorder),
	}
}

func (h *GameSeverEventHandler) OnAdd(ctx context.Context, obj interface{}) error {
	gs := gameserver.FromObject(obj)

	if err := h.Reconcile(ctx, h.logger.WithField("event", "added"), gs); err != nil {
		h.logger.Error(err)
	}

	return nil
}

func (h *GameSeverEventHandler) OnUpdate(ctx context.Context, _ interface{}, newObj interface{}) error {
	gs := gameserver.FromObject(newObj)

	if err := h.Reconcile(ctx, h.logger.WithField("event", "updated"), gs); err != nil {
		h.logger.Error(err)
	}

	return nil
}

func (h *GameSeverEventHandler) OnDelete(_ context.Context, obj interface{}) error {
	gs := obj.(*agonesv1.GameServer)
	h.logger.WithField("event", "deleted").Infof("%s/%s", gs.Namespace, gs.Name)

	return nil
}

func (h *GameSeverEventHandler) Reconcile(ctx context.Context, logger *logrus.Entry, gs *agonesv1.GameServer) error {
	if _, ok := gameserver.HasAnnotation(gs, gameserver.OctopsAnnotationIngressMode); !ok {
		logger.Infof("skipping %s/%s, annotation %s not present", gs.Namespace, gs.Name, gameserver.OctopsAnnotationIngressMode)
		return nil
	}

	//If a game server is in a Shutdown state it will not trigger reconcile
	if gameserver.IsShutdown(gs) {
		logger.WithField("event", "shutdown").Infof("%s/%s", gs.Namespace, gs.Name)

		return nil
	}

	//Only Scheduled, ReadyState and Ready game server states will trigger reconcile
	if gameserver.MustReconcile(gs) == false {
		msg := fmt.Sprintf("%s/%s/%s not reconciled, requires Scheduled, ReadyState or Ready state", gs.Namespace, gs.Name, gs.Status.State)
		logger.Info(msg)

		return nil
	}

	go func() {
		err := h.AsyncReconcileServiceAndIngress(ctx, logger, gs)
		if err != nil {
			h.logger.Error(err)
		}
	}()

	return nil
}

func (h *GameSeverEventHandler) AsyncReconcileServiceAndIngress(ctx context.Context, logger *logrus.Entry, gs *agonesv1.GameServer) error {

	_, err := h.serviceReconciler.Reconcile(ctx, gs)
	if err != nil {
		return errors.Wrapf(err, "failed to reconcile service %s", k8sutil.Namespaced(gs))
	}

	if delayMs, ok := gameserver.HasAnnotation(gs, gameserver.OctopsAnnotationIngressDelay); ok {
		delay, err := time.ParseDuration(delayMs)
		if err != nil {
			return errors.Wrapf(err, "failed to parse ingress delay duration %s, example: '3000ms'", delayMs)
		}
		time.Sleep(delay)
	}

	_, ingReconciled, err := h.ingressReconciler.Reconcile(ctx, gs)
	if err != nil {
		return errors.Wrapf(err, "failed to reconcile ingress %s", k8sutil.Namespaced(gs))
	}

	result, err := h.gameserverReconciler.Reconcile(ctx, gs)
	if err != nil {
		return errors.Wrapf(err, "failed to reconcile gameserver %s", k8sutil.Namespaced(gs))
	}

	if ingReconciled {
		msg := fmt.Sprintf("%s/%s", k8sutil.Namespaced(result), result.Status.State)
		//msg = fmt.Sprintf("%s/%s nothing to reconcile", k8sutil.Namespaced(result), result.Status.State)
		logger.WithFields(logrus.Fields{
			"reconciled": true,
			"ingress":    "created",
		}).Info(msg)
	}

	return nil
}
