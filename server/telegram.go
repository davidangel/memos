package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"

	"github.com/usememos/memos/api"
	"github.com/usememos/memos/common"
	"github.com/usememos/memos/plugin/telegram"
	"github.com/usememos/memos/store"
)

type telegramHandler struct {
	store *store.Store
}

func newTelegramHandler(store *store.Store) *telegramHandler {
	return &telegramHandler{store: store}
}

func (t *telegramHandler) BotToken(ctx context.Context) string {
	return t.store.GetSystemSettingValueOrDefault(&ctx, api.SystemSettingTelegramBotTokenName, "")
}

const (
	workingMessage = "Working on send your memo..."
	successMessage = "Success"
)

func (t *telegramHandler) MessageHandle(ctx context.Context, bot *telegram.Bot, message telegram.Message, blobs map[string][]byte) error {
	reply, err := bot.SendReplyMessage(ctx, message.Chat.ID, message.MessageID, workingMessage)
	if err != nil {
		return fmt.Errorf("fail to SendReplyMessage: %s", err)
	}

	var creatorID int
	userSettingList, err := t.store.FindUserSettingList(ctx, &api.UserSettingFind{
		Key: api.UserSettingTelegramUserIDKey,
	})
	if err != nil {
		_, err := bot.EditMessage(ctx, message.Chat.ID, reply.MessageID, fmt.Sprintf("Fail to find memo user: %s", err))
		return err
	}
	for _, userSetting := range userSettingList {
		var value string
		if err := json.Unmarshal([]byte(userSetting.Value), &value); err != nil {
			continue
		}

		if value == strconv.Itoa(message.From.ID) {
			creatorID = userSetting.UserID
		}
	}

	if creatorID == 0 {
		_, err := bot.EditMessage(ctx, message.Chat.ID, reply.MessageID, fmt.Sprintf("Please set your telegram userid %d in UserSetting of Memos", message.From.ID))
		return err
	}

	// create memo
	memoCreate := api.CreateMemoRequest{
		CreatorID:  creatorID,
		Visibility: api.Private,
	}

	if message.Text != nil {
		memoCreate.Content = *message.Text
	}
	if blobs != nil && message.Caption != nil {
		memoCreate.Content = *message.Caption
	}

	memoMessage, err := t.store.CreateMemo(ctx, convertCreateMemoRequestToMemoMessage(&memoCreate))
	if err != nil {
		_, err := bot.EditMessage(ctx, message.Chat.ID, reply.MessageID, fmt.Sprintf("failed to CreateMemo: %s", err))
		return err
	}

	if err := createMemoCreateActivity(ctx, t.store, memoMessage); err != nil {
		_, err := bot.EditMessage(ctx, message.Chat.ID, reply.MessageID, fmt.Sprintf("failed to createMemoCreateActivity: %s", err))
		return err
	}

	// create resources
	for filename, blob := range blobs {
		// TODO support more
		mime := "application/octet-stream"
		switch path.Ext(filename) {
		case ".jpg":
			mime = "image/jpeg"
		case ".png":
			mime = "image/png"
		}
		resourceCreate := api.ResourceCreate{
			CreatorID: creatorID,
			Filename:  filename,
			Type:      mime,
			Size:      int64(len(blob)),
			Blob:      blob,
			PublicID:  common.GenUUID(),
		}
		resource, err := t.store.CreateResource(ctx, &resourceCreate)
		if err != nil {
			_, err := bot.EditMessage(ctx, message.Chat.ID, reply.MessageID, fmt.Sprintf("failed to CreateResource: %s", err))
			return err
		}
		if err := createResourceCreateActivity(ctx, t.store, resource); err != nil {
			_, err := bot.EditMessage(ctx, message.Chat.ID, reply.MessageID, fmt.Sprintf("failed to createResourceCreateActivity: %s", err))
			return err
		}

		_, err = t.store.UpsertMemoResource(ctx, &api.MemoResourceUpsert{
			MemoID:     memoMessage.ID,
			ResourceID: resource.ID,
		})
		if err != nil {
			_, err := bot.EditMessage(ctx, message.Chat.ID, reply.MessageID, fmt.Sprintf("failed to UpsertMemoResource: %s", err))
			return err
		}
	}

	_, err = bot.EditMessage(ctx, message.Chat.ID, reply.MessageID, successMessage)
	return err
}
