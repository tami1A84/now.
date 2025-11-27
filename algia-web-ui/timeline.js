// =========================================================
// timeline.js - Nostrクライアント UI描画ロジック
// =========================================================

// --- 2. UI生成関数 ---
/**
 * 単一のイベントデータからHTML文字列を生成する
 * @param {object} event - 投稿イベントオブジェクト
 * @returns {string} - 生成された投稿カードのHTML文字列
 */
function createPostCard(event) {
    const tagsToProcess = event.tags || []; 
    const tagsHtml = tagsToProcess.map(tag => `#${tag}`).join(' ');

    const likeCount = event.likeCount || 0;     // ★LikeCountに変更★
    const repostCount = event.repostCount || 0; // ★新規追加★
    
    const fullPubkey = event.pubkey; 
    const displayUsername = event.displayUsername || fullPubkey.substring(0, 8); 
    const eventId = event.id;

    const iconUrl = event.iconUrl || `https://i.pravatar.cc/50?u=${fullPubkey}`;
    
    // いいね済み状態のクラス (簡易的にカウント > 0 で色を変える)
    const isLikedClass = likeCount > 0 ? 'liked' : '';

    return `
        <div class="post-card">
            <div class="post-header">
                <img src="${iconUrl}" alt="user icon" class="user-icon">
                <span class="username">@${displayUsername}</span>
            </div>
            <div class="post-body">
                <p class="post-text">${event.content}</p>
                <p class="hashtags">${tagsHtml}</p>
            </div>
            <div class="post-footer">
                <div class="post-actions">
                    <div 
                        class="like-action ${isLikedClass}" 
                        onclick="submitLike('${eventId}', '${fullPubkey}')"
                    >
                        <i class="fa-solid fa-heart like-icon"></i> 
                        <span class="like-count">${likeCount}</span>
                    </div>
                    <div 
                        class="repost-action" 
                        onclick="submitRepost('${eventId}', '${fullPubkey}')"
                    >
                        <i class="fa-solid fa-retweet repost-icon"></i> 
                        <span class="repost-count">${repostCount}</span>
                    </div>
                    <div 
                        class="reply-action" 
                        onclick="openReplyModal('${eventId}', '${fullPubkey}', '${displayUsername}')"
                    >
                        <i class="fa-solid fa-reply reply-icon"></i>
                    </div>
                </div>
            </div>
        </div>
    `;
}


// --- 3. メイン実行関数 ---
async function initializeTimeline() {
    console.log("タイムラインの描画を開始します...");
    
    const feedContainer = document.querySelector('.timeline-feed');
    
    const events = await fetchEvents(); 

    if (events.length === 0) {
        feedContainer.innerHTML = '<p class="secondary-text-color" style="text-align: center; padding: 20px;">表示する投稿がありません。</p>';
        return;
    }

    let allPostsHtml = '';
    events.forEach(event => {
        allPostsHtml += createPostCard(event);
    });
    
    feedContainer.innerHTML = allPostsHtml;
    console.log(`${events.length} 件の投稿を描画しました。`);
}


// DOMの読み込みが完了したら、タイムラインの初期化を開始する
document.addEventListener('DOMContentLoaded', initializeTimeline);

// =========================================================
// API連携用関数 (GET)
// =========================================================

async function fetchEvents() {
    const apiUrl = 'http://localhost:8080/api/v1/timeline'; 

    try {
        const response = await fetch(apiUrl);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        return await response.json(); 
    } catch (error) {
        console.error("APIからの取得に失敗しました。Goサーバーが起動しているか確認してください。", error);
        return []; 
    }
}


// =========================================================
// モーダル制御関数 (投稿/返信共通)
// =========================================================

/**
 * 投稿モーダルを表示する
 */
function openPostModal() {
    document.getElementById('postModal').style.display = 'flex';
    document.getElementById('postContent').value = ''; 
    document.getElementById('postStatusMessage').textContent = ''; 
}

/**
 * 投稿/返信モーダルを非表示にする
 * @param {string} type - 'post' または 'reply'
 */
function closeModal(type) {
    if (type === 'post') {
        document.getElementById('postModal').style.display = 'none';
    } else if (type === 'reply') {
        document.getElementById('replyModal').style.display = 'none';
    }
}

// =========================================================
// 新規投稿機能 (post, n)
// =========================================================
async function submitPost() {
    const content = document.getElementById('postContent').value.trim();
    const submitButton = document.getElementById('submitPostButton');
    const statusMessage = document.getElementById('postStatusMessage');

    if (!content) {
        statusMessage.textContent = '内容を入力してください。';
        statusMessage.style.color = 'red';
        return;
    }

    submitButton.disabled = true;
    submitButton.textContent = '投稿中...';
    statusMessage.textContent = 'リレーにイベントを送信しています...';
    statusMessage.style.color = 'var(--accent-color-blue)';

    const apiUrl = 'http://localhost:8080/api/v1/post';

    try {
        const response = await fetch(apiUrl, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ content: content }),
        });

        const result = await response.json();

        if (response.ok) {
            statusMessage.textContent = `投稿成功! Event ID: ${result.event_id.substring(0, 10)}...`;
            statusMessage.style.color = 'green';
            
            setTimeout(() => {
                closeModal('post');
                initializeTimeline(); 
            }, 1500);

        } else {
            statusMessage.textContent = `投稿エラー: ${result.error || '不明なエラー'}`;
            statusMessage.style.color = 'red';
        }

    } catch (error) {
        console.error("投稿API呼び出しに失敗しました:", error);
        statusMessage.textContent = '接続エラーが発生しました。サーバーを確認してください。';
        statusMessage.style.color = 'red';
    } finally {
        submitButton.disabled = false;
        submitButton.textContent = '投稿する';
    }
}


// =========================================================
// 返信機能 (reply, r)
// =========================================================

/**
 * 返信モーダルを表示し、対象情報をセットする
 */
function openReplyModal(id, fullPubkey, displayUsername) {
    document.getElementById('replyModal').style.display = 'flex';
    document.getElementById('replyContent').value = '';
    document.getElementById('replyStatusMessage').textContent = '';

    document.getElementById('replyTargetInfo').textContent = `@${displayUsername} への返信 (ID: ${id.substring(0, 10)}...)`;
    
    document.getElementById('hiddenReplyToId').value = id;
    document.getElementById('hiddenReplyToPubkey').value = fullPubkey; 
}


async function submitReply() {
    const content = document.getElementById('replyContent').value.trim();
    const replyToId = document.getElementById('hiddenReplyToId').value;
    const replyToPubkey = document.getElementById('hiddenReplyToPubkey').value;

    const submitButton = document.getElementById('submitReplyButton');
    const statusMessage = document.getElementById('replyStatusMessage');

    // ... (UI制御とAPI呼び出しロジックは変更なし)
    if (!content) {
        statusMessage.textContent = '内容を入力してください。';
        statusMessage.style.color = 'red';
        return;
    }
    
    submitButton.disabled = true;
    submitButton.textContent = '返信中...';
    statusMessage.textContent = 'リレーにイベントを送信しています...';
    statusMessage.style.color = 'var(--accent-color-blue)';

    const apiUrl = 'http://localhost:8080/api/v1/reply';

    try {
        const response = await fetch(apiUrl, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ 
                content: content,
                replyToId: replyToId,
                replyToPubkey: replyToPubkey 
            }),
        });

        const result = await response.json();

        if (response.ok) {
            statusMessage.textContent = `返信成功! Event ID: ${result.event_id.substring(0, 10)}...`;
            statusMessage.style.color = 'green';
            
            setTimeout(() => {
                closeModal('reply');
                initializeTimeline(); 
            }, 1500);

        } else {
            statusMessage.textContent = `返信エラー: ${result.error || '不明なエラー'}`;
            statusMessage.style.color = 'red';
        }

    } catch (error) {
        console.error("返信API呼び出しに失敗しました:", error);
        statusMessage.textContent = '接続エラーが発生しました。サーバーを確認してください。';
        statusMessage.style.color = 'red';
    } finally {
        submitButton.disabled = false;
        submitButton.textContent = '返信する';
    }
}


// =========================================================
// いいね機能 (like, l) (新規追加)
// =========================================================

async function submitLike(targetEventId, targetPubkey) {
    // 成功/失敗メッセージは画面上部に一時的に表示する
    const statusMessage = document.getElementById('globalStatusMessage');
    const apiUrl = 'http://localhost:8080/api/v1/like';

    statusMessage.textContent = 'いいねイベントを送信中...';
    statusMessage.style.color = 'var(--accent-color-blue)';
    statusMessage.style.display = 'block';

    try {
        const response = await fetch(apiUrl, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                targetEventId: targetEventId,
                targetPubkey: targetPubkey 
            }),
        });

        const result = await response.json();

        if (response.ok) {
            statusMessage.textContent = `いいね成功! Event ID: ${result.event_id.substring(0, 10)}...`;
            statusMessage.style.color = 'green';
            // タイムラインをリロードして LikeCount を更新
            initializeTimeline(); 

        } else {
            statusMessage.textContent = `いいねエラー: ${result.error || '不明なエラー'}`;
            statusMessage.style.color = 'red';
        }

    } catch (error) {
        console.error("いいねAPI呼び出しに失敗しました:", error);
        statusMessage.textContent = '接続エラーが発生しました。サーバーを確認してください。';
        statusMessage.style.color = 'red';
    } finally {
        setTimeout(() => {
            statusMessage.style.display = 'none';
        }, 1500);
    }
}


// =========================================================
// リポスト機能 (repost, b) (新規追加)
// =========================================================

async function submitRepost(targetEventId, targetPubkey) {
    const statusMessage = document.getElementById('globalStatusMessage');
    const apiUrl = 'http://localhost:8080/api/v1/repost';

    statusMessage.textContent = 'リポストイベントを送信中...';
    statusMessage.style.color = 'var(--accent-color-blue)';
    statusMessage.style.display = 'block';

    try {
        const response = await fetch(apiUrl, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                targetEventId: targetEventId,
                targetPubkey: targetPubkey 
            }),
        });

        const result = await response.json();

        if (response.ok) {
            statusMessage.textContent = `リポスト成功! Event ID: ${result.event_id.substring(0, 10)}...`;
            statusMessage.style.color = 'green';
            // タイムラインをリロードして RepostCount を更新
            initializeTimeline(); 

        } else {
            statusMessage.textContent = `リポストエラー: ${result.error || '不明なエラー'}`;
            statusMessage.style.color = 'red';
        }

    } catch (error) {
        console.error("リポストAPI呼び出しに失敗しました:", error);
        statusMessage.textContent = '接続エラーが発生しました。サーバーを確認してください。';
        statusMessage.style.color = 'red';
    } finally {
        setTimeout(() => {
            statusMessage.style.display = 'none';
        }, 1500);
    }
}


// DOMContentLoaded イベントリスナーの再設定
document.addEventListener('DOMContentLoaded', () => {
    // 投稿アイコン
    const postIcon = document.querySelector('.app-header .fa-pen-to-square');
    if (postIcon) {
        postIcon.addEventListener('click', openPostModal); // 関数名を修正
    }
    
    // モーダル内の「投稿する」ボタン
    const submitPostButton = document.getElementById('submitPostButton');
    if (submitPostButton) {
        submitPostButton.addEventListener('click', submitPost);
    }

    // 返信機能のイベントリスナー
    const submitReplyButton = document.getElementById('submitReplyButton');
    if (submitReplyButton) {
        submitReplyButton.addEventListener('click', submitReply);
    }
    
    // グローバルなステータスメッセージ要素を追加 (いいね、リポスト用)
    const topBar = document.querySelector('.top-bar');
    if (topBar && !document.getElementById('globalStatusMessage')) {
        const statusDiv = document.createElement('div');
        statusDiv.id = 'globalStatusMessage';
        statusDiv.className = 'post-status'; // post-statusスタイルを流用
        statusDiv.style.textAlign = 'center';
        statusDiv.style.padding = '10px 0';
        statusDiv.style.position = 'absolute';
        statusDiv.style.top = '60px'; // ヘッダーの下あたり
        statusDiv.style.left = '0';
        statusDiv.style.right = '0';
        statusDiv.style.zIndex = '100';
        statusDiv.style.display = 'none';
        
        document.querySelector('.screen').prepend(statusDiv);
    }
});
