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

package open_im_sdk

import (
	"context"
	"errors"
	"github.com/openimsdk/openim-sdk-core/v3/internal/business"
	conv "github.com/openimsdk/openim-sdk-core/v3/internal/conversation_msg"
	"github.com/openimsdk/openim-sdk-core/v3/internal/file"
	"github.com/openimsdk/openim-sdk-core/v3/internal/friend"
	"github.com/openimsdk/openim-sdk-core/v3/internal/full"
	"github.com/openimsdk/openim-sdk-core/v3/internal/group"
	"github.com/openimsdk/openim-sdk-core/v3/internal/interaction"
	"github.com/openimsdk/openim-sdk-core/v3/internal/login"
	"github.com/openimsdk/openim-sdk-core/v3/internal/third"
	"github.com/openimsdk/openim-sdk-core/v3/internal/user"
	"github.com/openimsdk/openim-sdk-core/v3/open_im_sdk_callback"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/ccontext"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/common"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/db/db_interface"
	"github.com/openimsdk/openim-sdk-core/v3/pkg/utils"
	"strings"
	"sync"
)

// Caller is an interface that defines the SDK's basic and message sending caller.
type Caller interface {
	BaseCaller(funcName interface{}, base open_im_sdk_callback.Base, args ...interface{})
	SendMessageCaller(funcName interface{}, messageCallback open_im_sdk_callback.SendMsgCallBack, args ...interface{})
}

var (

	// Client-independent user class
	UserForSDK *LoginMgr
)

type LoginMgr struct {
	friend       *friend.Friend
	group        *group.Group
	conversation *conv.Conversation
	user         *user.User
	file         *file.File
	business     *business.Business

	full         *full.Full
	db           db_interface.DataBase
	longConnMgr  *interaction.LongConnMgr
	msgSyncer    *interaction.MsgSyncer
	third        *third.Third
	token        string
	loginUserID  string
	connListener open_im_sdk_callback.OnConnListener

	loginTime int64

	w           sync.Mutex
	loginStatus int

	groupListener               open_im_sdk_callback.OnGroupListener
	friendListener              open_im_sdk_callback.OnFriendshipListener
	conversationListener        open_im_sdk_callback.OnConversationListener
	advancedMsgListener         open_im_sdk_callback.OnAdvancedMsgListener
	batchMsgListener            open_im_sdk_callback.OnBatchMsgListener
	userListener                open_im_sdk_callback.OnUserListener
	signalingListener           open_im_sdk_callback.OnSignalingListener
	signalingListenerFroService open_im_sdk_callback.OnSignalingListener
	businessListener            open_im_sdk_callback.OnCustomBusinessListener

	conversationCh     chan common.Cmd2Value
	cmdWsCh            chan common.Cmd2Value
	heartbeatCmdCh     chan common.Cmd2Value
	pushMsgAndMaxSeqCh chan common.Cmd2Value
	loginMgrCh         chan common.Cmd2Value

	ctx    context.Context
	cancel context.CancelFunc
	info   *ccontext.GlobalConfig
}

func NewLoginMgr() *LoginMgr {
	return &LoginMgr{}
}

// CheckResourceLoad checks the SDK is resource load status.
func CheckResourceLoad(uSDK *login.LoginMgr, funcName string) error {
	if uSDK == nil {
		return utils.Wrap(errors.New("CheckResourceLoad failed uSDK == nil "), "")
	}
	if funcName == "" {
		return nil
	}
	parts := strings.Split(funcName, ".")
	if parts[len(parts)-1] == "Login-fm" {
		return nil
	}
	if uSDK.Friend() == nil || uSDK.User() == nil || uSDK.Group() == nil || uSDK.Conversation() == nil ||
		uSDK.Full() == nil {
		return utils.Wrap(errors.New("CheckResourceLoad failed, resource nil "), "")
	}
	return nil
}

type name struct {
}
