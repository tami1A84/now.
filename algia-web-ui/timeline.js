// =========================================================
// timeline.js - Nostrクライアント UI描画ロジック
// =========================================================

// --- 1. ダミーデータ (Goバックエンドが返すJSONの代わり) ---
const DUMMY_EVENTS = [
    {
        id: '1',
        pubkey: 'nostr_girl',
        content: 'Go言語でNostrウェブアプリ開発、始めたい！API GatewayとLambdaでデコってもいいよなデコってもうかな...',
        tags: ['Go', 'Nostr', 'WebDev'],
        iconUrl: 'https://i.pravatar.cc/50?u=nostr_girl',
        zapCount: 15
    },
    {
        id: '2',
        pubkey: 'Zap_Flow',
        content: '新しいNWCウォルト、セットチップ完了！すぐに投げ銭をテストめよう。便り分ける！',
        tags: ['NWC', 'Bitcoin', 'Lightning'],
        iconUrl: 'https://i.pravatar.cc/50?u=zap_flow',
        zapCount: 8
    },
    {
        id: '3',
        pubkey: 'dev_chan',
        content: '分散型ソーシャルメディアの未来は明るい。Nostrはその一端を担うだろう。UI改善も頑張るぞ！',
        tags: ['Nostr', 'Decentralization', 'WebDev'],
        iconUrl: 'https://i.pravatar.cc/50?u=dev_chan',
        zapCount: 22
    }
];


// --- 2. UI生成関数 ---
/**
 * 単一のイベントデータからHTML文字列を生成する
 * @param {object} event - 投稿イベントオブジェクト
 * @returns {string} - 生成された投稿カードのHTML文字列
 */
function createPostCard(event) {
    // 既存のHTML構造をテンプレートリテラルで定義
    
    // ★★★ 修正箇所: event.tags が null または undefined の場合に備えて空の配列（[]）を設定 ★★★
    const tagsToProcess = event.tags || []; 
    const tagsHtml = tagsToProcess.map(tag => `#${tag}`).join(' ');

    return `
        <div class="post-card">
            <div class="post-header">
                <img src="${event.iconUrl}" alt="user icon" class="user-icon">
                <span class="username">@${event.pubkey}</span>
            </div>
            <div class="post-body">
                <p class="post-text">${event.content}</p>
                <p class="hashtags">${tagsHtml}</p>
            </div>
            <div class="post-footer">
                <i class="fa-solid fa-bolt zap-icon"></i> 
                </div>
        </div>
    `;
}


// --- 3. メイン実行関数 ---
/**
 * タイムラインを初期化し、イベントを取得・描画する
 */
async function initializeTimeline() {
    console.log("タイムラインの描画を開始します...");
    
    // 描画先のコンテナを取得
    const feedContainer = document.querySelector('.timeline-feed');
    
    // APIからデータを取得する（Goサーバーが起動している前提）
    const events = await fetchEvents(); 

    if (events.length === 0) {
        feedContainer.innerHTML = '<p class="secondary-text-color" style="text-align: center; padding: 20px;">表示する投稿がありません。</p>';
        return;
    }

    // 取得したイベントをループし、HTMLを生成してコンテナに追加
    let allPostsHtml = '';
    events.forEach(event => {
        allPostsHtml += createPostCard(event);
    });
    
    // 一括でDOMに追加することで描画パフォーマンスを向上させる
    feedContainer.innerHTML = allPostsHtml;
    console.log(`${events.length} 件の投稿を描画しました。`);
}


// DOMの読み込みが完了したら、タイムラインの初期化を開始する
document.addEventListener('DOMContentLoaded', initializeTimeline);

// =========================================================
// API連携用関数
// =========================================================

async function fetchEvents() {
    // GoサーバーのURL
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
