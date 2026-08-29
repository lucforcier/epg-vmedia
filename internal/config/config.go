package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Provider             string
	APIURL               string
	ImageURL             string
	SiteID               int
	AppName              string
	Language             string
	DataDir              string
	Host                 string
	Port                 string
	EPGBaseURL           string
	EPGXMLPath           string
	ImageCacheMaxAgeDays int

	SchedulerEnabled  bool
	SchedulerTime     string
	SchedulerTimezone string

	ChannelRefreshEnabled bool
	ChannelRefreshDay     int
	ChannelRefreshTime    string

	ImageCleanupEnabled bool
	ImageCleanupDay     int
	ImageCleanupTime    string
}

func Load() (Config, error) {
	cfg := Config{
		Provider:           os.Getenv("EPG_PROVIDER"),
		APIURL:             os.Getenv("VMEDIA_API_URL"),
		ImageURL:           os.Getenv("VMEDIA_IMAGE_URL"),
		AppName:            os.Getenv("VMEDIA_APP_NAME"),
		Language:           os.Getenv("VMEDIA_LANGUAGE"),
		DataDir:            os.Getenv("EPG_DATA_DIR"),
		Host:               os.Getenv("EPG_HOST"),
		Port:               os.Getenv("EPG_PORT"),
		EPGBaseURL:         os.Getenv("EPG_BASE_URL"),
		EPGXMLPath:         os.Getenv("EPG_XML_PATH"),
		SchedulerTime:      os.Getenv("EPG_SCHEDULER_TIME"),
		SchedulerTimezone:  os.Getenv("EPG_SCHEDULER_TIMEZONE"),
		ChannelRefreshTime: os.Getenv("EPG_CHANNEL_REFRESH_TIME"),
		ImageCleanupTime:   os.Getenv("IMAGE_CACHE_CLEANUP_TIME"),
	}

	if cfg.Provider == "" {
		cfg.Provider = "fizz"
	}

	if cfg.APIURL == "" {
		cfg.APIURL = "https://a2.vmedia.ca"
	}

	if cfg.ImageURL == "" {
		cfg.ImageURL = "https://images.vmedia.ca/api/images"
	}

	if cfg.AppName == "" {
		cfg.AppName = "F-GOOGLE-PLAY"
	}

	if cfg.Language == "" {
		cfg.Language = "fra"
	}

	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}

	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}

	if cfg.Port == "" {
		cfg.Port = "5580"
	}

	if cfg.EPGXMLPath == "" {
		cfg.EPGXMLPath = cfg.DataDir + "/epg.xml"
	}

	if cfg.EPGBaseURL == "" {
		cfg.EPGBaseURL = "http://localhost:" + cfg.Port
	}

	// ------------------------------------------------------------
	// Cache des images
	// ------------------------------------------------------------

	cacheAgeText := os.Getenv("IMAGE_CACHE_MAX_AGE_DAYS")

	if cacheAgeText == "" {
		cacheAgeText = "90"
	}

	cacheAge, err := strconv.Atoi(cacheAgeText)
	if err != nil {
		return Config{}, fmt.Errorf(
			"IMAGE_CACHE_MAX_AGE_DAYS invalide %q: %w",
			cacheAgeText,
			err,
		)
	}

	if cacheAge <= 0 {
		return Config{}, fmt.Errorf(
			"IMAGE_CACHE_MAX_AGE_DAYS doit être supérieur à zéro",
		)
	}

	cfg.ImageCacheMaxAgeDays = cacheAge

	// ------------------------------------------------------------
	// VMedia Site ID
	// ------------------------------------------------------------

	siteIDText := os.Getenv("VMEDIA_SITE_ID")

	if siteIDText == "" {
		siteIDText = "176"
	}

	siteID, err := strconv.Atoi(siteIDText)
	if err != nil {
		return Config{}, fmt.Errorf(
			"VMEDIA_SITE_ID invalide %q: %w",
			siteIDText,
			err,
		)
	}

	cfg.SiteID = siteID

	// ------------------------------------------------------------
	// Scheduler
	// ------------------------------------------------------------

	schedulerEnabledText := os.Getenv("EPG_SCHEDULER_ENABLED")

	if schedulerEnabledText == "" {
		schedulerEnabledText = "true"
	}

	schedulerEnabled, err := strconv.ParseBool(schedulerEnabledText)
	if err != nil {
		return Config{}, fmt.Errorf(
			"EPG_SCHEDULER_ENABLED invalide %q: %w",
			schedulerEnabledText,
			err,
		)
	}

	cfg.SchedulerEnabled = schedulerEnabled

	if cfg.SchedulerTime == "" {
		cfg.SchedulerTime = "03:00"
	}

	if _, err := time.Parse("15:04", cfg.SchedulerTime); err != nil {
		return Config{}, fmt.Errorf(
			"EPG_SCHEDULER_TIME invalide %q: %w",
			cfg.SchedulerTime,
			err,
		)
	}

	if cfg.SchedulerTimezone == "" {
		cfg.SchedulerTimezone = "America/Toronto"
	}

	if _, err := time.LoadLocation(cfg.SchedulerTimezone); err != nil {
		return Config{}, fmt.Errorf(
			"EPG_SCHEDULER_TIMEZONE invalide %q: %w",
			cfg.SchedulerTimezone,
			err,
		)
	}

	// ------------------------------------------------------------
	// Rafraîchissement hebdomadaire des chaînes et logos
	// ------------------------------------------------------------

	channelRefreshEnabledText := os.Getenv(
		"EPG_CHANNEL_REFRESH_ENABLED",
	)

	if channelRefreshEnabledText == "" {
		channelRefreshEnabledText = "true"
	}

	channelRefreshEnabled, err := strconv.ParseBool(
		channelRefreshEnabledText,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"EPG_CHANNEL_REFRESH_ENABLED invalide %q: %w",
			channelRefreshEnabledText,
			err,
		)
	}

	cfg.ChannelRefreshEnabled = channelRefreshEnabled

	channelRefreshDayText := os.Getenv(
		"EPG_CHANNEL_REFRESH_DAY",
	)

	if channelRefreshDayText == "" {
		channelRefreshDayText = "1"
	}

	channelRefreshDay, err := strconv.Atoi(
		channelRefreshDayText,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"EPG_CHANNEL_REFRESH_DAY invalide %q: %w",
			channelRefreshDayText,
			err,
		)
	}

	if channelRefreshDay < 1 || channelRefreshDay > 7 {
		return Config{}, fmt.Errorf(
			"EPG_CHANNEL_REFRESH_DAY doit être compris entre 1 et 7",
		)
	}

	cfg.ChannelRefreshDay = channelRefreshDay

	if cfg.ChannelRefreshTime == "" {
		cfg.ChannelRefreshTime = "03:30"
	}

	if _, err := time.Parse(
		"15:04",
		cfg.ChannelRefreshTime,
	); err != nil {
		return Config{}, fmt.Errorf(
			"EPG_CHANNEL_REFRESH_TIME invalide %q: %w",
			cfg.ChannelRefreshTime,
			err,
		)
	}

	// ------------------------------------------------------------
	// Nettoyage du cache des images
	// ------------------------------------------------------------

	imageCleanupEnabledText := os.Getenv(
		"IMAGE_CACHE_CLEANUP_ENABLED",
	)

	if imageCleanupEnabledText == "" {
		imageCleanupEnabledText = "true"
	}

	imageCleanupEnabled, err := strconv.ParseBool(
		imageCleanupEnabledText,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"IMAGE_CACHE_CLEANUP_ENABLED invalide %q: %w",
			imageCleanupEnabledText,
			err,
		)
	}

	cfg.ImageCleanupEnabled = imageCleanupEnabled

	imageCleanupDayText := os.Getenv(
		"IMAGE_CACHE_CLEANUP_DAY",
	)

	if imageCleanupDayText == "" {
		imageCleanupDayText = "1"
	}

	imageCleanupDay, err := strconv.Atoi(
		imageCleanupDayText,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"IMAGE_CACHE_CLEANUP_DAY invalide %q: %w",
			imageCleanupDayText,
			err,
		)
	}

	if imageCleanupDay < 1 || imageCleanupDay > 7 {
		return Config{}, fmt.Errorf(
			"IMAGE_CACHE_CLEANUP_DAY doit être compris entre 1 et 7",
		)
	}

	cfg.ImageCleanupDay = imageCleanupDay

	if cfg.ImageCleanupTime == "" {
		cfg.ImageCleanupTime = "04:00"
	}

	if _, err := time.Parse(
		"15:04",
		cfg.ImageCleanupTime,
	); err != nil {
		return Config{}, fmt.Errorf(
			"IMAGE_CACHE_CLEANUP_TIME invalide %q: %w",
			cfg.ImageCleanupTime,
			err,
		)
	}

	return cfg, nil
}
