package communication

import (
	"log/slog"
	"sync"

	"github.com/Roma7-7-7/sso-notifier/models"
)

type MessageSender interface {
	Send(chatID int64, msg string) error
}

type NotificationRepository interface {
	GetAll() ([]models.Notification, error)
	Delete(id int) error
}

type Service struct {
	repo   NotificationRepository
	sender MessageSender

	notifyTaskMx sync.Mutex
}

func (s *Service) SendMessage(chatID int64, msg string) error {
	return s.sender.Send(chatID, msg)
}

func (s *Service) SendQueuedNotifications() {
	s.notifyTaskMx.Lock()
	defer s.notifyTaskMx.Unlock()

	ns, err := s.repo.GetAll()
	if err != nil {
		slog.Error("failed to get queued notifications", "error", err)
		return
	}
	for _, n := range ns {
		subID := slog.Int64("subscriberID", n.Target)
		notificationID := slog.Int("notificationID", n.ID)

		if err = s.sender.Send(n.Target, n.Msg); err != nil {
			slog.Error("failed to send notification", "error", err, subID, notificationID)
			continue
		}
		if err = s.repo.Delete(n.ID); err != nil {
			slog.Error("failed to delete notification from queue", "error", err, subID, notificationID)
			continue
		}
		slog.Debug("notification sent", subID, notificationID)
	}
}

func NewNotificationService(repo NotificationRepository, sender MessageSender) *Service {
	return &Service{
		repo:   repo,
		sender: sender,
	}
}
