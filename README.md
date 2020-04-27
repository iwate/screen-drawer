# screen-drawer
[Virtual Office](https://github.com/iwate/VirtualOfiice) と接続するスクリーンドロワーアプリケーションです。

Virtual Officeでデスクトップ共有を開始して人のスクリーン上に他者が書きこめるようにします。

## Features

起動するとWebSocketサーバが起動し、定型のメッセージを送ることでスクリーン上描画します。

## Disclaimer

WebSocketサーバは意図的にオリジンチェックを外しています。つまり、このWebSocketサーバを起動すると、あらゆるWebサイトから繋ぐことができます。

これは、昨今のZoomの脆弱性と同じような構造になっていることを示します。

オリジンチェックを外した理由としては、起動パラメータにしろ環境変数にしろ、設定するように説明することが少々手間に感じたことに尽きます。

このWebSocketサーバの機能としては、スクリーンに描画することしか機能がないため、この仕様をクラッカーに使用されたとしても、最悪スクリーン上を勝手に描画される程度であると認識しています。
その際は、アプリケーションを閉じてください。

以上の点が心配な方は、オリジンチェックの部分も作りこんであるため、コメントアウトを外しご自身でビルドしてご使用いただけますようお願い申し上げます。

## How to Use

### Windows

1. [Release](https://github.com/iwate/screen-drawer/releases) から最新版のexeをダウンロード
2. ダウンロードしたexeを起動
3. `cert.cer` ファイルと `key.pem` ファイルが作成されていることを確認
4. `cert.cer` をダブルクリック
5.  証明書をインストール
6. 保存場所：ローカルコンピュータ で次へ
7. 証明書をすべて次のストアへ配置する：信頼されたルート証明書 で次へ
8. 完了

### Mac

Contribute募集中
