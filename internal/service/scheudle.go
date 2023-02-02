package service

import (
	"time"
)

type ShutdownsService interface {
	RefreshShutdownsTable()
}

type SubscriptionService interface {
	SendUpdates()
}

type CommunicationService interface {
	SendQueuedNotifications()
}

const refreshTableInterval = 5 * time.Minute
const sendUpdatesInterval = 5 * time.Second
const notificationInterval = 5 * time.Minute

type Scheduler struct {
	shutdownsService    ShutdownsService
	subscriptionService SubscriptionService
	notificationService CommunicationService
}

func (s *Scheduler) RefreshTable() {
	for {
		s.shutdownsService.RefreshShutdownsTable()
		time.Sleep(refreshTableInterval)
	}
}

func (s *Scheduler) SendUpdates() {
	for {
		s.subscriptionService.SendUpdates()
		time.Sleep(sendUpdatesInterval)
	}
}

func (s *Scheduler) SendNotificationsTask() {
	for {
		s.notificationService.SendQueuedNotifications()
		time.Sleep(notificationInterval)
	}
}

func NewScheduler(
	shutdownsService ShutdownsService, subscriptionService SubscriptionService, notificationService CommunicationService,
) *Scheduler {

	return &Scheduler{
		shutdownsService:    shutdownsService,
		subscriptionService: subscriptionService,
		notificationService: notificationService,
	}
}
