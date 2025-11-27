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

// ★★★ テスト用秘密鍵/公開鍵 ★★★
const TEST_PRIVATE_KEY_HEX = "9e32f41e0653d9e96f1d2e0b51079d36a3f019f5630d664b413c1c9c0494f71a" 
const TEST_PUBLIC_KEY_HEX = "8d57d0d04c4b57494f10874c431b0a8c2d1033230a133a8a3a0c4f83b6f00f0d"  
// -------------------------------------------------------------

// Event 構造体 (いいねとリポストのカウンターを追加)
type Event struct {
	ID              string   `json:"id"`
	Pubkey          string   `json:"pubkey"` 
	DisplayUsername string   `json:"displayUsername"` 
	Content         string   `json:"content"`
	Tags            []string `json:"tags"`
	IconURL         string   `json:"iconUrl"`
	LikeCount       int      `json:"likeCount"`   // ★ZapCountをLikeCountに変更★
	RepostCount     int      `json:"repostCount"` // ★新規追加★
}

// Profile 構造体
type Profile struct {
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// PostRequest 構造体
type PostRequest struct {
	Content string `json:"content"`
}

// ReplyRequest 構造体
type ReplyRequest struct {
	Content string `json:"content"`
	ReplyToID string `json:"replyToId"` 
	ReplyToPubkey string `json:"replyToPubkey"` 
}

// LikeRequest / RepostRequest 構造体 (ターゲットIDと公開鍵のみ)
type InteractionRequest struct {
	TargetEventID string `json:"targetEventId"`
	TargetPubkey  string `json:"targetPubkey"` 
}

// --------------------------------------------------------
// 2. ヘルパー関数群
// --------------------------------------------------------

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
// 3. Nostrイベント取得ロジック
// --------------------------------------------------------

func fetchTimelineEvents() []Event {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second) 
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

	// PubkeyとEvent IDの収集
	pubkeys := make(map[string]struct{})
	eventIDs := make(map[string]struct{})
	for _, ev := range rawEvents {
		pubkeys[ev.PubKey] = struct{}{}
		eventIDs[ev.ID] = struct{}{} 
	}
	var pubkeyList []string
	for pk := range pubkeys { pubkeyList = append(pubkeyList, pk) }
	var idList []string
	for id := range eventIDs { idList = append(idList, id) }

	// 3.2. Kind 0イベント（プロフィール）の取得
	filter0 := nostr.Filter{
		Kinds: []int{0},
		Authors: pubkeyList,
	}
	rawProfiles, _ := relay.QuerySync(ctx, filter0) 
	
	// 3.3. Kind 7 (Reaction/Like) と Kind 6 (Repost) の取得
	interactionFilter := nostr.Filter{
		Kinds: []int{6, 7}, // Kind 6 (Repost) と Kind 7 (Reaction)
		Tags:  nostr.TagMap{"e": idList}, 
	}
	rawInteractions, _ := relay.QuerySync(ctx, interactionFilter) 

	// 3.4. プロフィールマップの作成
	profileMap := make(map[string]Profile)
	for _, profEv := range rawProfiles {
		var p Profile
		if err := json.Unmarshal([]byte(profEv.Content), &p); err == nil {
			profileMap[profEv.PubKey] = p
		}
	}
	
	// 3.5. Like数とRepost数の集計 (Kind 7 は content: "+" のみカウント)
	likeCounts := make(map[string]int) 
	repostCounts := make(map[string]int)
	for _, interactionEv := range rawInteractions {
		tag := interactionEv.Tags.GetFirst([]string{"e"}) 
		
		if tag != nil && len(*tag) >= 2 {
			eventID := (*tag)[1] 
			
			if interactionEv.Kind == nostr.KindReaction && interactionEv.Content == "+" {
				// Kind 7 で content が "+" のものを Like としてカウント
				likeCounts[eventID]++
			} else if interactionEv.Kind == nostr.KindRepost {
				// Kind 6 を Repost としてカウント
				repostCounts[eventID]++
			}
		}
	}


	// 3.6. 最終的なイベントリストの構築と情報のマッピング
	var events []Event
	for _, ev := range rawEvents {
		
		displayUsername := ev.PubKey[:8] 
		iconUrl := "https://i.pravatar.cc/50?u=" + ev.PubKey 
		
		if p, found := profileMap[ev.PubKey]; found {
			if p.Name != "" {
				displayUsername = p.Name
			}
			if p.Picture != "" {
				iconUrl = p.Picture
			}
		}

		// カウンターの取得
		likeCount := likeCounts[ev.ID] 
		repostCount := repostCounts[ev.ID]

		events = append(events, Event{
			ID:      ev.ID,
			Pubkey:  ev.PubKey,     
			DisplayUsername: displayUsername, 
			Content: ev.Content,
			Tags:    extractHashtags(ev.Tags.GetAll([]string{"t"})), 
			IconURL: iconUrl,
			LikeCount: likeCount, // 更新
			RepostCount: repostCount, // 新規
		})
	}
	
	log.Printf("Kind 1: %d 件, Kind 0: %d 件, Kind 7/6: %d 件を処理しました。", len(rawEvents), len(rawProfiles), len(rawInteractions))
	return events
}

// --------------------------------------------------------
// 4. APIエンドポイントのハンドラ
// --------------------------------------------------------

func postNoteHandler(c *gin.Context) {
	var req PostRequest
	// ... (投稿ロジックは変更なし)
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "リクエストボディのパースに失敗しました"})
		return
	}
	// ...
	ev := nostr.Event{
		PubKey: TEST_PUBLIC_KEY_HEX,
		CreatedAt: nostr.Now(),
		Kind: nostr.KindTextNote,
		Content: req.Content,
		Tags: nostr.Tags{}, 
	}
	// ... (署名と公開ロジックは変更なし)
	if err := ev.Sign(TEST_PRIVATE_KEY_HEX); err != nil {
		log.Printf("イベント署名エラー: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "イベントの署名に失敗しました"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	relay, err := nostr.RelayConnect(ctx, defaultRelay)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "リレー接続に失敗しました"})
		return
	}
	defer relay.Close()
	
	err = relay.Publish(ctx, ev) 
	if err != nil { 
		log.Printf("イベント公開エラー: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "イベントの公開に失敗しました"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "投稿が成功しました", "event_id": ev.ID})
}

func replyNoteHandler(c *gin.Context) {
	var req ReplyRequest
	// ... (返信ロジックは変更なし)
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "リクエストボディのパースに失敗しました"})
		return
	}
	// ...
	ev := nostr.Event{
		PubKey: TEST_PUBLIC_KEY_HEX,
		CreatedAt: nostr.Now(),
		Kind: nostr.KindTextNote,
		Content: req.Content,
		Tags: nostr.Tags{
			nostr.Tag{"e", req.ReplyToID, ""},
			nostr.Tag{"p", req.ReplyToPubkey, ""},
		},
	}
	// ... (署名と公開ロジックは変更なし)
	if err := ev.Sign(TEST_PRIVATE_KEY_HEX); err != nil {
		log.Printf("イベント署名エラー: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "イベントの署名に失敗しました"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	relay, err := nostr.RelayConnect(ctx, defaultRelay)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "リレー接続に失敗しました"})
		return
	}
	defer relay.Close()
	
	err = relay.Publish(ctx, ev)
	if err != nil {
		log.Printf("イベント公開エラー: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "イベントの公開に失敗しました"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "返信が成功しました", "event_id": ev.ID})
}

// Like (Kind 7) イベントを作成し、リレーに送信するハンドラ (新規追加)
func likeNoteHandler(c *gin.Context) {
	var req InteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "リクエストボディのパースに失敗しました"})
		return
	}

	if req.TargetEventID == "" || req.TargetPubkey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TargetEventID, TargetPubkey は必須です"})
		return
	}

	// 1. Kind 7 (Reaction) イベントの作成
	ev := nostr.Event{
		PubKey: TEST_PUBLIC_KEY_HEX,
		CreatedAt: nostr.Now(),
		Kind: nostr.KindReaction, // Kind 7
		Content: "+", // Like は content: "+" を使用する
		Tags: nostr.Tags{
			nostr.Tag{"e", req.TargetEventID, ""},
			nostr.Tag{"p", req.TargetPubkey, ""},
		},
	}

	// 2. 署名と公開 (postNoteHandler と共通ロジック)
	if err := ev.Sign(TEST_PRIVATE_KEY_HEX); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "イベントの署名に失敗しました"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	relay, err := nostr.RelayConnect(ctx, defaultRelay)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "リレー接続に失敗しました"})
		return
	}
	defer relay.Close()
	
	err = relay.Publish(ctx, ev)
	if err != nil {
		log.Printf("イベント公開エラー: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "イベントの公開に失敗しました"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Likeイベントが公開されました", "event_id": ev.ID})
}

// Repost (Kind 6) イベントを作成し、リレーに送信するハンドラ (新規追加)
func repostNoteHandler(c *gin.Context) {
	var req InteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "リクエストボディのパースに失敗しました"})
		return
	}

	if req.TargetEventID == "" || req.TargetPubkey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TargetEventID, TargetPubkey は必須です"})
		return
	}

	// 1. Kind 6 (Repost) イベントの作成
	ev := nostr.Event{
		PubKey: TEST_PUBLIC_KEY_HEX,
		CreatedAt: nostr.Now(),
		Kind: nostr.KindRepost, // Kind 6
		Content: "", // RepostのContentは通常空
		Tags: nostr.Tags{
			nostr.Tag{"e", req.TargetEventID, ""},
			nostr.Tag{"p", req.TargetPubkey, ""},
		},
	}

	// 2. 署名と公開 (postNoteHandler と共通ロジック)
	if err := ev.Sign(TEST_PRIVATE_KEY_HEX); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "イベントの署名に失敗しました"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	relay, err := nostr.RelayConnect(ctx, defaultRelay)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "リレー接続に失敗しました"})
		return
	}
	defer relay.Close()
	
	err = relay.Publish(ctx, ev)
	if err != nil {
		log.Printf("イベント公開エラー: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "イベントの公開に失敗しました"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Repostイベントが公開されました", "event_id": ev.ID})
}

func getTimelineHandler(c *gin.Context) {
	events := fetchTimelineEvents()
	c.JSON(http.StatusOK, events) 
}

// --------------------------------------------------------
// 5. サーバー起動
// --------------------------------------------------------

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

	// ルーティング
	r.GET("/api/v1/timeline", getTimelineHandler)
	r.POST("/api/v1/post", postNoteHandler) 
	r.POST("/api/v1/reply", replyNoteHandler) 
	r.POST("/api/v1/like", likeNoteHandler)   // ★新規追加★
	r.POST("/api/v1/repost", repostNoteHandler) // ★新規追加★

	if err := r.Run(":8080"); err != nil {
		panic("サーバー起動に失敗しました: " + err.Error())
	}
}
