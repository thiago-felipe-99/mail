package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	errMaxEntrySize       = errors.New("entry is to big")
	errInvalidContentType = errors.New("obeject has a invalid Content Type")
	errSmallBuffer        = errors.New("unable to get all template")
)

type cache struct {
	data              *bigcache.BigCache
	bucket            string
	minio             *minio.Client
	maxEntrySize      int64
	validContentTypes []string
}

func newCache(configs *configurations) (*cache, error) {
	const megabyte = 1000 * 1000

	dataConfig := bigcache.Config{
		Shards:             configs.Cache.Shards,
		LifeWindow:         time.Duration(configs.Cache.LifeWindow) * time.Minute,
		CleanWindow:        time.Duration(configs.Cache.CleanWindow) * time.Minute,
		MaxEntriesInWindow: configs.Cache.AvgEntries,
		MaxEntrySize:       configs.Cache.AvgEntrySize * megabyte,
		HardMaxCacheSize:   configs.Cache.MaxSize,
		StatsEnabled:       configs.Cache.Statics,
		Verbose:            configs.Cache.Verbose,
	}

	data, err := bigcache.New(context.Background(), dataConfig)
	if err != nil {
		return nil, fmt.Errorf("erro creating BigCache: %w", err)
	}

	host := fmt.Sprintf("%s:%d", configs.Minio.Host, configs.Minio.Port)
	minioOptions := &minio.Options{
		Creds: credentials.NewStaticV4(
			configs.Minio.AccessKey,
			configs.Minio.SecretKey,
			"",
		),
	}

	minio, err := minio.New(host, minioOptions)
	if err != nil {
		return nil, fmt.Errorf("error creating Minio client: %w", err)
	}

	return &cache{
		data:              data,
		bucket:            configs.Cache.Bucket,
		minio:             minio,
		maxEntrySize:      int64(configs.Cache.MaxEntrySize) * megabyte,
		validContentTypes: nil,
	}, nil
}

func validContentType(contentType string, contentTypes []string) bool {
	if len(contentTypes) == 0 {
		return true
	}

	for _, validContentType := range contentTypes {
		if validContentType == contentType {
			return true
		}
	}

	return false
}

func (cache *cache) getFileFromMinio(name string) ([]byte, error) {
	object, err := cache.minio.GetObject(
		context.Background(),
		cache.bucket,
		name,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("error getting object from minio: %w", err)
	}

	objectInfo, err := object.Stat()
	if err != nil {
		return nil, fmt.Errorf("error getting object status: %w", err)
	}

	if !validContentType(objectInfo.ContentType, cache.validContentTypes) {
		return nil, fmt.Errorf("%w, %s", errInvalidContentType, objectInfo.ContentType)
	}

	if objectInfo.Size > cache.maxEntrySize {
		return nil, errMaxEntrySize
	}

	file := make([]byte, objectInfo.Size)

	_, err = object.Read(file)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("error reading file: %w", err)
	} else if err == nil {
		return nil, errSmallBuffer
	}

	err = cache.data.Set(name, file)
	if err != nil {
		return nil, fmt.Errorf("error setting file on cache: %w", err)
	}

	return file, nil
}

func (cache *cache) getFile(name string) ([]byte, error) {
	file, err := cache.data.Get(name)
	if err != nil {
		if errors.Is(err, bigcache.ErrEntryNotFound) {
			return cache.getFileFromMinio(name)
		}

		return nil, fmt.Errorf("error getting file from minio: %w", err)
	}

	return file, nil
}

type templateCache struct {
	cache
}

func newTemplate(configs *configurations) (*templateCache, error) {
	const megabyte = 1000 * 1000

	dataConfig := bigcache.Config{
		Shards:             configs.Template.Shards,
		LifeWindow:         0,
		CleanWindow:        0,
		MaxEntriesInWindow: configs.Template.AvgEntries,
		MaxEntrySize:       configs.Template.AvgEntrySize * megabyte,
		HardMaxCacheSize:   configs.Template.MaxSize,
		StatsEnabled:       configs.Template.Statics,
		Verbose:            configs.Template.Verbose,
	}

	data, err := bigcache.New(context.Background(), dataConfig)
	if err != nil {
		return nil, fmt.Errorf("erro creating BigCache: %w", err)
	}

	host := fmt.Sprintf("%s:%d", configs.Minio.Host, configs.Minio.Port)
	minioOptions := &minio.Options{
		Creds: credentials.NewStaticV4(
			configs.Minio.AccessKey,
			configs.Minio.SecretKey,
			"",
		),
	}

	minio, err := minio.New(host, minioOptions)
	if err != nil {
		return nil, fmt.Errorf("error creating Minio client: %w", err)
	}

	template := &templateCache{
		cache: cache{
			data:              data,
			bucket:            configs.Template.Bucket,
			minio:             minio,
			maxEntrySize:      int64(configs.Template.MaxEntrySize) * megabyte,
			validContentTypes: []string{"text/markdown"},
		},
	}

	return template, nil
}

func (template *templateCache) setAll() {
	options := minio.ListObjectsOptions{
		WithVersions: false,
		WithMetadata: true,
		Prefix:       "",
		Recursive:    true,
	}

	templatesQuantity := 0

	for info := range template.cache.minio.ListObjects(context.Background(), template.bucket, options) {
		if info.Err != nil {
			log.Printf("[ERROR] - Error getting '%s' template info: %s", info.Key, info.Err)

			continue
		}

		_, err := template.cache.getFileFromMinio(info.Key)
		if err != nil {
			log.Printf("[ERROR] - Error setting '%s' template: %s", info.Key, err)
		} else {
			templatesQuantity++
		}
	}

	log.Printf("[INFO] - %d templates on cache", templatesQuantity)
}

func (template *templateCache) get(name string) ([]byte, error) {
	return template.cache.getFile(name)
}
