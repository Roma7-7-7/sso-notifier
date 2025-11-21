package telegram

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Dev                      bool          `envconfig:"DEV" default:"false"`
	GroupsCount              int           `envconfig:"GROUPS_COUNT" default:"12"`
	DBPath                   string        `envconfig:"DB_PATH" default:"data/sso-notifier.db"`
	RefreshShutdownsInterval time.Duration `envconfig:"REFRESH_SHUTDOWNS_INTERVAL" default:"5m"`
	NotifyInterval           time.Duration `envconfig:"NOTIFY_INTERVAL" default:"5m"`
	NotifyUpcomingInterval   time.Duration `envconfig:"NOTIFY_UPCOMING_INTERVAL" default:"1m"`
	ScheduleURL              string        `envconfig:"SCHEDULE_URL" default:"https://oblenergo.cv.ua/shutdowns/"`
	NotificationsStateTTL    time.Duration `envconfig:"NOTIFICATIONS_STATE_TTL" default:"24h"`
	CleanupInterval          time.Duration `envconfig:"CLEANUP_INTERVAL" default:"1h"`
	AlertsTTL                time.Duration `envconfig:"ALERTS_TTL" default:"24h"`
	TelegramToken            string        `envconfig:"TELEGRAM_TOKEN"`
}

func NewConfig(ctx context.Context) (*Config, error) {
	res := &Config{}

	err := envconfig.Process("", res)
	if err != nil {
		return nil, fmt.Errorf("envconfig process: %w", err)
	}

	if res.Dev {
		return res, nil
	}
	res.TelegramToken, err = getSSMToken(ctx)
	if err != nil {
		return nil, err
	}

	if res.TelegramToken == "" {
		return nil, errors.New("telegram token is required")
	}

	return res, nil
}

func getSSMToken(ctx context.Context) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("load aws config: %w", err)
	}
	ssmClient := ssm.NewFromConfig(cfg)

	param, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String("/sso-notifier-bot/prod/telegram-token"),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("get SSM token: %w", err)
	}
	if param.Parameter.Value == nil {
		return "", errors.New("SSM Token not found")
	}

	return *param.Parameter.Value, nil
}
