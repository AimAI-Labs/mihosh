package config

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/spf13/viper"
)

const subscriptionUsageKey = "subscription_usage"

type subscriptionUsageRecord struct {
	UID        string                    `mapstructure:"uid" yaml:"uid"`
	SourceHash string                    `mapstructure:"source_hash" yaml:"source_hash"`
	Usage      profile.SubscriptionUsage `mapstructure:"usage" yaml:"usage"`
}

func subscriptionSourceHash(source profile.SubSource) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(source.URL))))
}

func subscriptionUsageRecords(subscriptions []profile.Profile) []subscriptionUsageRecord {
	records := []subscriptionUsageRecord{}
	for _, sub := range subscriptions {
		if sub.Source.Kind == profile.SourceRemote && sub.Usage != nil {
			records = append(records, subscriptionUsageRecord{
				UID: sub.UID, SourceHash: subscriptionSourceHash(sub.Source), Usage: *sub.Usage,
			})
		}
	}
	return records
}

func restoreSubscriptionUsage(reader *viper.Viper, cfg *Config) {
	var records []subscriptionUsageRecord
	if err := reader.UnmarshalKey(subscriptionUsageKey, &records); err != nil {
		records = nil // Inline usage still works if the backup is malformed.
	}
	saved := make(map[string]subscriptionUsageRecord, len(records))
	for _, record := range records {
		saved[record.UID] = record
	}
	for i := range cfg.Subs {
		sub := &cfg.Subs[i]
		if sub.Source.Kind != profile.SourceRemote {
			continue
		}
		record, present := saved[sub.UID]
		matches := present && record.SourceHash == subscriptionSourceHash(sub.Source)
		switch {
		case sub.Usage == nil && matches:
			sub.Usage = &record.Usage
		case sub.Usage != nil && (!matches || !reflect.DeepEqual(*sub.Usage, record.Usage)):
			cfg.subscriptionUsageNeedsBackup = true
		}
	}
}

var errSubscriptionUsageBackedUp = errors.New("subscription usage backup is up to date")

// PreserveSubscriptionUsage seeds the backup on startup before older clients can rewrite subs.
func PreserveSubscriptionUsage() error {
	err := Update(func(cfg *Config) error {
		if !cfg.subscriptionUsageNeedsBackup {
			return errSubscriptionUsageBackedUp
		}
		return nil
	})
	if errors.Is(err, errSubscriptionUsageBackedUp) {
		return nil
	}
	return err
}
