package main

import (
  "fmt"
  //"io"
  "net/http"
  "os"
  "path/filepath"
  "strconv"
)

const uploadDir = "./uploads"

func main() {
  // 保存フォルダ作成
  if err := os.MkdirAll(uploadDir, 0755); err != nil {
    fmt.Println("フォルダの作成に失敗したのだ:", err)
    return
  }

  // ルートアクセス 入力フォーム 兼 保存処理）
  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    // POST（フォームの送信）が送られてきたときの処理
    if r.Method == http.MethodPost {

      // サイズチェック, バイト単位, 1<<20 で 1MB
			err := r.ParseMultipartForm(32 << 20)
			if err == nil {
			  // 画像ファイルを取得
			  // 中身、ファイル名などのメタな情報、エラーってのは良くしらない。
			  file, header, err := r.FormFile("image")
				
				if err == nil {
					
          // 何が起きても必ず最後にクローズするように
					defer file.Close()

					// 512 バイトの入れ物を作る
					buf := make([]byte, 512)

          // file の中身を buf に送る。buf が 512バイトしかないので、先頭の 512 バイトだけ
					// 入る。_ は実際に buf に送れたバイト数になる。
					// Go は使わない変数があるとエラーになるので _ で捨てると名言するらしい。
					_, err = file.Read(buf)

					if err == nil {
				    // MIME タイプを取得
						contentType := http.DetectContentType(buf)

            // さっき file を 512 バイト読み進めてしまっているので先頭に戻す。
						_, err = file.Seek(0, 0)
            
            // 拡張子決め。該当しないなら .bin
            if err == nil {
              ext := ".bin"
              switch contentType {
              case "image/jpeg": 
                ext = ".jpg"
              case "image/png": 
                ext = ".png"
              case "image/gif": 
                ext = ".gif"
              case "image/webp": 
                ext = ".webp"
              }
            
              // パス決め
              nextNum := getNextFileNumber(uploadDir)
              filename := fmt.Sprintf("%04d%s", nextNum, ext)
              savePath := filepath.Join(uploadDir, filename)

              // 書き込み
              dst, err := os.Create(savePath)
						  if err == nil {
						  	defer dst.Close()
						  	_, _ = io.Copy(dst, file)
						  }
					  }
				  }
			  }

      // フォームから "text" 名前の入力値を取得
      text := r.FormValue("text")
      if text != "" {
        // フォルダ内にあるファイルを調べて、次の連番を決めるのだ
        nextNum := getNextFileNumber(uploadDir)
        
        // 4桁のファイル名（例: 0001.txt）を作るのだ
        filename := fmt.Sprintf("%04d.txt", nextNum)
        filepath := filepath.Join(uploadDir, filename)

        // ファイルにテキストを書き込んで保存するのだ
        err := os.WriteFile(filepath, []byte(text), 0644)
        if err != nil {
          http.Error(w, "ファイルの保存に失敗したのだ", http.StatusInternalServerError)
          return
        }
      }
      // 保存が終わったら、同じページにリロードして綺麗に戻すのだ
      http.Redirect(w, r, "/", http.StatusSeeOther)
      return
    }

    // GET（普通にアクセスしたとき）は、入力フォームを表示するのだ
    html := `
      <!DOCTYPE html>
      <html>
      <head><meta charset="utf-8"><title>テキスト連番保存</title></head>
      <body>
      <h2>テキストを連番で保存するのだ</h2>
      <form method="POST" action="/" enctype="multipart/form-data">
      <textarea name="text" rows="4" cols="40" placeholder="ここに文字を入力...">
			</textarea>
			<input type="file" name="image" accept="image/*"></input>
			<button type="submit" >実行</button>
			</body>
			</html>
    `

    fmt.Fprint(w, html)
  })

  fmt.Println("サーバーを起動するのだ！ http://localhost:8080 にアクセスしてね")
  if err := http.ListenAndServe(":8080", nil); err != nil {
    fmt.Println("エラーなのだ:", err)
  }
}
// フォルダ内を覗いて、次の連番（1, 2, 3...）を計算する関数なのだ
func getNextFileNumber(dir string) int {
  files, err := os.ReadDir(dir)
  if err != nil {
    return 1
  }
  maxNum := 0
  for _, file := range files {
    if file.IsDir() {
      continue
    }
    // 拡張子を除いたファイル名（例: "0001"）を数字に変換してみるのだ
    name := file.Name()
    if len(name) >= 4 {
      ext := filepath.Ext(name)
      base := name[0 : len(name)-len(ext)]
      if num, err := strconv.Atoi(base); err == nil {
        if num > maxNum {
          maxNum = num
        }
      }
    }
  }
  return maxNum + 1
}

