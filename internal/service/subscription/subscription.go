package subscription

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/Roma7-7-7/sso-notifier/models"
)

const GroupsCount = 18
const subscriptionsLimit = 1000

type MessageSender interface {
	Send(chatID int64, text string) error
}

type ShutdownsService interface {
	GetShutdownsTable() (models.ShutdownsTable, bool, error)
	RefreshShutdownsTable()
}

type Repository interface {
	Size() (int, error)
	Exists(chatID int64) (bool, error)
	Get(chatID int64) (models.Subscription, bool, error)
	GetAll() ([]models.Subscription, error)
	Put(sub models.Subscription) (models.Subscription, error)
	Purge(chatID int64) error
}

type Service struct {
	repo             Repository
	shutdownsService ShutdownsService
	sender           MessageSender

	sendUpdatesMx sync.Mutex
}

func (s *Service) GroupsCount() int {
	return GroupsCount
}

func (s *Service) IsSubscribed(chatID int64) (bool, error) {
	exists, err := s.repo.Exists(chatID)
	if err != nil {
		return false, fmt.Errorf("failed to check if subscription exists: %w", err)
	}
	return exists, nil
}

func (s *Service) GetSubscriptions() ([]models.Subscription, error) {
	subs, err := s.repo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions: %w", err)
	}
	return subs, nil
}

func (s *Service) SubscribeToGroup(chatID int64, groupNum string) (models.Subscription, error) {
	size, err := s.repo.Size()
	if err != nil {
		return models.Subscription{}, fmt.Errorf("failed to get number of subscribers: %w", err)
	}

	sub, exists, err := s.repo.Get(chatID)
	if err != nil {
		return models.Subscription{}, fmt.Errorf("failed to get subscription: %w", err)
	}

	if !exists {
		if size >= subscriptionsLimit {
			return models.Subscription{}, models.ErrSubscriptionsLimitReached
		}
		sub = models.Subscription{
			ChatID: chatID,
		}
	}

	sub.Groups = map[string]string{
		groupNum: "",
	}
	sub, err = s.repo.Put(sub)
	if err != nil {
		return models.Subscription{}, fmt.Errorf("failed to put subscription: %w", err)
	}

	if !exists {
		zap.L().Debug("new subscriber", zap.Int64("chatID", chatID))
	}

	return sub, nil
}

func (s *Service) Unsubscribe(chatID int64) error {
	return s.repo.Purge(chatID)
}

func (s *Service) SendUpdates() {
	s.sendUpdatesMx.Lock()
	defer s.sendUpdatesMx.Unlock()

	table, ok, err := s.shutdownsService.GetShutdownsTable()
	if err != nil {
		zap.L().Error("failed to get shutdowns table", zap.Error(err))
		return
	}
	if !ok {
		// table is not ready yet
		return
	}
	grouped := make(map[string]models.ShutdownGroup)
	for k, v := range table.Groups {
		grouped[k] = v
	}

	subs, err := s.repo.GetAll()
	if err != nil {
		zap.L().Error("failed to get subscriptions", zap.Error(err))
		return
	}

	for _, sub := range subs {
		s.processSubscription(sub, table, grouped)
	}
}

func (s *Service) processSubscription(
	sub models.Subscription, table models.ShutdownsTable, grouped map[string]models.ShutdownGroup) {

	msgs := make([]string, 0)

	chatID := sub.ChatID
	zapChatID := zap.Int64("chatID", chatID)
	for groupNum, hash := range sub.Groups {
		// Hack to make sure updates for new day will be sent even if there is no changes in schedule
		newHash := grouped[groupNum].Hash(fmt.Sprintf("%s:", table.Date))
		if hash == newHash {
			continue
		}

		gropuedPeriod, groupedStatuses := join(table.Periods, grouped[groupNum].Items)
		cutPeriod, cutStatuses := cutByKyivTime(gropuedPeriod, groupedStatuses)
		msg, err := renderGroup(groupNum, cutPeriod, cutStatuses)
		if err != nil {
			zap.L().Error("failed to render group message", zap.Error(err), zapChatID, zap.String("group", groupNum))
			return
		}
		msgs = append(msgs, msg)
		sub.Groups[groupNum] = newHash
	}

	if len(msgs) == 0 {
		return
	}

	msg, err := renderMessage(table.Date, msgs)
	if err != nil {
		zap.L().Error("failed to render message", zap.Error(err), zapChatID)
		return
	}
	if err := s.sender.Send(chatID, msg); err != nil {
		zap.L().Error("failed to send message", zap.Error(err), zapChatID)
		return
	}

	if _, err := s.repo.Put(sub); err != nil {
		zap.L().Error("failed to update subscription", zap.Error(err), zapChatID)
		return
	}
}

var kyivTime = time.FixedZone("Kyiv", 2*60*60) //nolint:gomnd

func join(periods []models.Period, statuses []models.Status) ([]models.Period, []models.Status) {
	groupedPeriod := make([]models.Period, 0)
	groupedStatus := make([]models.Status, 0)

	currentFrom := periods[0].From
	currentTo := periods[0].To
	currentStatus := statuses[0]
	for i := 1; i < len(periods); i++ {
		if statuses[i] == currentStatus {
			currentTo = periods[i].To
			continue
		}
		groupedPeriod = append(groupedPeriod, models.Period{From: currentFrom, To: currentTo})
		groupedStatus = append(groupedStatus, currentStatus)
		currentFrom = periods[i].From
		currentTo = periods[i].To
		currentStatus = statuses[i]
	}
	groupedPeriod = append(groupedPeriod, models.Period{From: currentFrom, To: currentTo})
	groupedStatus = append(groupedStatus, currentStatus)

	return groupedPeriod, groupedStatus
}

func cutByKyivTime(periods []models.Period, items []models.Status) ([]models.Period, []models.Status) {
	currentKyivDateTime := time.Now().In(kyivTime).Format("15:04")

	cutPeriods := make([]models.Period, 0)
	cutItems := make([]models.Status, 0)
	for i := 0; i < len(periods); i++ {
		if periods[i].To > currentKyivDateTime {
			cutPeriods = append(cutPeriods, periods[i])
			cutItems = append(cutItems, items[i])
		}
	}

	return cutPeriods, cutItems
}

func NewSubscriptionService(repo Repository, shutdownsService ShutdownsService, sender MessageSender) *Service {
	return &Service{
		repo:             repo,
		shutdownsService: shutdownsService,
		sender:           sender,
	}
}
