// Copyright © 2023 OpenIM SDK. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package login

import (
	"context"
	"github.com/OpenIMSDK/protocol/push"
	"github.com/OpenIMSDK/tools/log"
	"github.com/openimsdk/openim-sdk-core/v3/internal/business"
	conv "github.com/openimsdk/openim-sdk-core/v3/internal/conversation_msg"
	"github.com/openimsdk/openim-sdk-core/v3/internal/file"
	"github.com/openimsdk/openim-sdk-core/v3/internal/friend"
	"github.com/openimsdk/openim-sdk-core/v3/internal/full"
	"github.com/openimsdk/openim-sdk-core/v3/internal/group"
	"github.com/openimsdk/openim-sdk-core/v3/internal/interaction"
	"github.com/openimsdk/openim-sdk-core/v3/internal/third"
	"github.com/openimsdk/openim-sdk-core/v3/internal/user"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/ccontext"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/common"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/constant"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/db"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/db/db_interface"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/db/model_struct"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/sdkerrs"
	"sync"
	"time"
)

const (
	Logout = iota + 1
	Logging
	Logged
)

type InitLogin struct {
	friend       *friend.Friend
	group        *group.Group
	conversation *conv.Conversation
	user         *user.User
	file         *file.File
	business     *business.Business
	third        *third.Third

	full        *full.Full
	db          db_interface.DataBase
	longConnMgr *interaction.LongConnMgr
	msgSyncer   *interaction.MsgSyncer

	loginMgrCh chan common.Cmd2Value

	w           sync.Mutex
	loginStatus int

	ctx    context.Context
	cancel context.CancelFunc
	info   *ccontext.GlobalConfig
}

func NewInitLogin(friend *friend.Friend, group *group.Group, conversation *conv.Conversation, user *user.User,
	file *file.File, business *business.Business, third *third.Third, full *full.Full, db db_interface.DataBase,
	longConnMgr *interaction.LongConnMgr, info *ccontext.GlobalConfig, msgSyncer *interaction.MsgSyncer,
	loginMgrCh chan common.Cmd2Value) *InitLogin {
	return &InitLogin{friend: friend, group: group, conversation: conversation, user: user,
		file: file, business: business, third: third, full: full, db: db, longConnMgr: longConnMgr,
		info: info, msgSyncer: msgSyncer, loginMgrCh: loginMgrCh}
}

func (u *InitLogin) getLoginStatus(_ context.Context) int {
	u.w.Lock()
	defer u.w.Unlock()
	return u.loginStatus
}
func (u *InitLogin) setLoginStatus(status int) {
	u.w.Lock()
	defer u.w.Unlock()
	u.loginStatus = status
}
func (u *InitLogin) BaseCtx() context.Context {
	return u.ctx
}

func (u *InitLogin) Exit() {
	u.cancel()
}

func (u *InitLogin) checkSendingMessage(ctx context.Context) {
	sendingMessages, err := u.db.GetAllSendingMessages(ctx)
	if err != nil {
		log.ZError(ctx, "GetAllSendingMessages failed", err)
	}
	for _, message := range sendingMessages {
		tableMessage, err := u.db.GetMessage(ctx, message.ConversationID, message.ClientMsgID)
		if err != nil {
			log.ZError(ctx, "GetMessage failed", err, "message", message)
			continue
		}
		if tableMessage.Status == constant.MsgStatusSending {
			err := u.db.UpdateMessage(ctx, message.ConversationID,
				&model_struct.LocalChatLog{ClientMsgID: message.ClientMsgID, Status: constant.MsgStatusSendFailed})
			if err != nil {
				log.ZError(ctx, "UpdateMessage failed", err, "tableMessage", tableMessage)
			} else {
				err := u.db.DeleteSendingMessage(ctx, message.ConversationID, message.ClientMsgID)
				if err != nil {
					log.ZError(ctx, "DeleteSendingMessage failed", err, "tableMessage", tableMessage)
				}
			}

		}
	}
}

func (u *InitLogin) login(ctx context.Context, userID, token string) error {
	if u.getLoginStatus(ctx) == Logged {
		return sdkerrs.ErrLoginRepeat
	}
	u.setLoginStatus(Logging)
	u.info.UserID = userID
	u.info.Token = token
	log.ZInfo(ctx, "login start... ", "userID", userID, "token", token)
	t1 := time.Now()
	//u.token = token
	//u.loginUserID = userID
	var err error
	u.db, err = db.NewDataBase(ctx, userID, u.info.DataDir, int(u.info.LogLevel))
	u.checkSendingMessage(ctx)
	if err != nil {
		return sdkerrs.ErrSdkInternal.Wrap("init database " + err.Error())
	}
	u.setLoginUserID(userID)
	if err := u.msgSyncer.LoadSeq(ctx); err != nil {
		log.ZError(ctx, "loadSeq err", err)
		return err
	}
	log.ZDebug(ctx, "NewDataBase ok", "userID", userID, "dataDir", u.info.DataDir, "login cost time", time.Since(t1))
	//u.loginTime = time.Now().UnixNano() / 1e6
	log.ZDebug(ctx, "forcedSynchronization success...", "login cost time: ", time.Since(t1))
	u.run(ctx)
	u.setLoginStatus(Logged)
	log.ZInfo(ctx, "login success...", "login cost time: ", time.Since(t1))
	return nil
}

func (u *InitLogin) setLoginUserID(userID string) {
	u.user.SetLoginUserID(userID)
	u.friend.SetLoginUserID(userID)
	u.file.SetLoginUserID(userID)
	u.group.SetLoginUserID(userID)
	u.conversation.SetLoginUserID(userID)
	u.third.SetLoginUserID(userID)
	u.msgSyncer.SetLoginUserID(userID)
}

func (u *InitLogin) run(ctx context.Context) {
	u.longConnMgr.Run(ctx)
	go u.msgSyncer.DoListener(ctx)
	go common.DoListener(u.conversation, ctx)
	go u.group.DeleteGroupAndMemberInfo(ctx)
	go u.logoutListener(ctx)
}

func (u *InitLogin) logoutListener(ctx context.Context) {
	for {
		select {
		case <-u.loginMgrCh:
			log.ZDebug(ctx, "logoutListener exit")
			err := u.logout(ctx, true)
			if err != nil {
				log.ZError(ctx, "logout error", err)
			}
		case <-ctx.Done():
			log.ZInfo(ctx, "logoutListener done sdk logout.....")
			return
		}
	}

}

// token error recycle recourse, kicked not recycle
func (u *InitLogin) logout(ctx context.Context, isTokenValid bool) error {
	if !isTokenValid {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		err := u.longConnMgr.SendReqWaitResp(ctx, &push.DelUserPushTokenReq{UserID: u.info.UserID, PlatformID: u.info.PlatformID}, constant.LogoutMsg, &push.DelUserPushTokenResp{})
		if err != nil {
			log.ZWarn(ctx, "TriggerCmdLogout server recycle resources failed...", err)
		} else {
			log.ZDebug(ctx, "TriggerCmdLogout server recycle resources success...")
		}
	}
	u.Exit()
	_ = u.db.Close(u.ctx)
	// user object must be rest  when user logout
	u.initResources()
	log.ZDebug(ctx, "TriggerCmdLogout client success...", "isTokenValid", isTokenValid)
	return nil
}
