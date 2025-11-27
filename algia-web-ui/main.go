package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	nostr "github.com/nbd-wtf/go-nostr" 
)

// --------------------------------------------------------
// 1. 定数とデータ構造の定義
// --------------------------------------------------------

const defaultRelay = "wss://relay.damus.io" 

// Event 構造体 (フロントエンドの timeline.js に合わせる)
type Event struct {
	ID      string   `json:"id"`
	Pubkey  string   `json:"pubkey"` // ここにはユーザー名（name）が入るようになる
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	IconURL string   `json:"iconUrl"`
	ZapCount int     `json:"zapCount"`
}

// Profile 構造体 (Kind 0 の content JSONをパースするために使用)
type Profile struct {
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// --------------------------------------------------------
// 2. ヘルパー関数群
// --------------------------------------------------------

// NostrのTags構造体（[][]string）から、ハッシュタグの値（tag[1]）のみを抽出して[]stringを返す
func extractHashtags(tags nostr.Tags) []string {
	var hashtags []string
	for _, tag := range tags {
		if len(tag) >= 2 && tag[0] == "t" {
			hashtags = append(hashtags, tag[1])
		}
	}
	return hashtags
}

// --------------------------------------------------------
// 3. Nostrイベント取得ロジック (コア統合部分 - Kind 1 + Kind 0)
// --------------------------------------------------------

func fetchTimelineEvents() []Event {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(ctx, defaultRelay)
	if err != nil {
		log.Printf("リレー接続エラー (%s): %v", defaultRelay, err)
		return []Event{}
	}
	defer relay.Close() 

	// 3.1. Kind 1イベント（投稿）の取得
	filter1 := nostr.Filter{
		Kinds: []int{1}, 
		Limit: 10,
	}
	rawEvents, err := relay.QuerySync(ctx, filter1)
	if err != nil {
		log.Printf("Kind 1 イベント取得エラー: %v", err)
		return []Event{}
	}

	// 3.2. プロフィール（Kind 0）取得のためのPubkey収集
	pubkeys := make(map[string]struct{})
	for _, ev := range rawEvents {
		pubkeys[ev.PubKey] = struct{}{}
	}

	// 3.3. Kind 0イベント（プロフィール）の取得
	var pubkeyList []string
	for pk := range pubkeys {
		pubkeyList = append(pubkeyList, pk)
	}

	filter0 := nostr.Filter{
		Kinds: []int{0},
		Authors: pubkeyList, // 投稿者全員のプロフィールをリクエスト
	}
    // 最新のプロフィールのみを取得するため、QuerySyncでリクエスト
	rawProfiles, err := relay.QuerySync(ctx, filter0) 
	if err != nil {
		log.Printf("Kind 0 イベント取得エラー: %v", err)
	}
	
    // 3.4. プロフィールマップの作成 (Pubkey -> Profile)
	profileMap := make(map[string]Profile)
	for _, profEv := range rawProfiles {
        // 同じPubkeyのイベントが複数ある場合、最新のもの（CreatedAtが最大）を採用すべきだが、ここではシンプルに上書き
        // JSON contentをパース
		var p Profile
		if err := json.Unmarshal([]byte(profEv.Content), &p); err == nil {
			profileMap[profEv.PubKey] = p
		}
	}

	// 3.5. 最終的なイベントリストの構築とプロフィール情報のマッピング
	var events []Event
	for _, ev := range rawEvents {
		
		username := ev.PubKey[:8] // デフォルトはPubkeyの先頭
		iconUrl := "https://i.pravatar.cc/50?u=" + ev.PubKey // デフォルトはダミーアイコン
		
        if p, found := profileMap[ev.PubKey]; found {
			// プロフィールが見つかった場合
			if p.Name != "" {
				username = p.Name
			}
			if p.Picture != "" {
				iconUrl = p.Picture
			}
		}

		events = append(events, Event{
			ID:      ev.ID,
			Pubkey:  username, // ユーザー名に置き換え
			Content: ev.Content,
			Tags:    extractHashtags(ev.Tags.GetAll([]string{"t"})), 
			IconURL: iconUrl, // 実際のアイコンURLに置き換え
			ZapCount: 0,
		})
	}
	
	log.Printf("Kind 1 イベント %d 件を取得し、Kind 0 プロフィール %d 件を処理しました。", len(rawEvents), len(rawProfiles))
	return events
}

// --------------------------------------------------------
// 4. APIエンドポイントのハンドラ、サーバー起動 (変更なし)
// --------------------------------------------------------

func getTimelineHandler(c *gin.Context) {
	events := fetchTimelineEvents()
	c.JSON(http.StatusOK, events) 
}

func main() {
	r := gin.Default()

    // CORS設定
    r.Use(func(c *gin.Context) {
        c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(http.StatusNoContent)
            return
        }
        c.Next()
    })

	r.GET("/api/v1/timeline", getTimelineHandler)

	if err := r.Run(":8080"); err != nil {
		panic("サーバー起動に失敗しました: " + err.Error())
	}
}
