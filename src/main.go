package main

import (
  "crypto/subtle"
  "encoding/json"
  "fmt"
  "io"
  "net/http"
  "os"
  "path/filepath"
  "regexp"
  "sort"
  "strconv"
  "strings"

  "golang.org/x/crypto/bcrypt"
)

type Config struct {
  UploadDir string `json:"uploadDir"`
  AuthUser  string `json:"authUser"`
  AuthPass  string `json:"authPass"`
  Port      string `json:"port"`
}

var cfg Config

const uploadDir = "./uploads"

// for basic 認証
const authUser = "a"
const authPass = "a"

func main() {
  loadConfig()
  // 保存フォルダ作成
  if err := os.MkdirAll(uploadDir, 0755); err != nil {
    fmt.Println("フォルダの作成に失敗したのだ:", err)
    return
  }

  // http://hoge.com/uploads で uploadDir にアクセスするようにする
  http.HandleFunc("/uploads/", basicAuth(func(w http.ResponseWriter, r *http.Request) {
    target := strings.TrimPrefix(r.URL.Path, "/uploads/")
    http.ServeFile(w, r, filepath.Join(uploadDir, target))
  }))

  // ルートアクセス 入力フォーム 兼 保存処理）
  http.HandleFunc("/", basicAuth(func(w http.ResponseWriter, r *http.Request) {
    // POST（フォームの送信）が送られてきたときの処理
    if r.Method == http.MethodPost {
      
      // 削除
      if deleteFile := r.FormValue("delete_file"); deleteFile != "" {

        // 他のフォルダにアクセスさせないためのセキュリティ対策
        // ファイル名しか取れない。ディレクトリは取れない
        safeName := filepath.Base(deleteFile)
        targetPath := filepath.Join(uploadDir, safeName)
        _ = os.Remove(targetPath)
        
        // ファイル番号振り直し
        renumberFiles(uploadDir)

        http.Redirect(w, r, "/", http.StatusSeeOther)
        return
      }

      // サイズチェック, バイト単位, 1<<20 で 1MB
      err := r.ParseMultipartForm(32 << 20)
      if err == nil {
        // 画像ファイルを取得
        // 中身、ファイル名などのメタな情報、エラーってのは良くしらない。
        file, _, err := r.FormFile("image")
        
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

    // 保存してあるファイルを流し込む

    var contentHtml strings.Builder
    files, err := os.ReadDir(uploadDir)
    if err == nil {
      // ファイルのソート
      sort.Slice(files, func(i, j int) bool {
        return files[i].Name() < files[j].Name()
      })

      for _, f := range files {
        if f.IsDir() {
          continue
        }
        name := f.Name()
        filePath := filepath.Join(uploadDir, name)
        contentHtml.WriteString("<div style='border: 3px solid #ccc; margin: 10px 0; padding: 10px;'>")
        contentHtml.WriteString(fmt.Sprintf("<strong>%s:</strong><br>", name))

        ext := strings.ToLower(filepath.Ext(name))
        if ext == ".txt" {
          data, err := os.ReadFile(filePath)
          if err == nil {
            linkedText := linkifyURLs(string(data))
            contentHtml.WriteString(fmt.Sprintf("<pre style='white-space: pre-wrap; font-family: inherit;'>%s</pre><br>", linkedText))
          }
        } else if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" {
          contentHtml.WriteString(fmt.Sprintf("<br><img src='/uploads/%s' style='max-width: 80%; height: auto;'><br>", name))
        }
        // 削除ボタン
        contentHtml.WriteString(fmt.Sprintf(`
          <!--
          <form method="POST" action="/" style="display:inline; float:right;" onsubmit="return confirm('やるぞ?');">
          -->
          <form method="POST" action="/" style="display:inline; float:left;">
          <input type="hidden" name="delete_file" value="%s"></input>
          <button type="submit" style="background-color: #ffcccc; border: 1px solid #cc0000; cursor: pointer; border-radius: 3px;">削除</button>
          </form>
        `, name) )
        contentHtml.WriteString("</div>")
      }
    }

    // GET（普通にアクセスしたとき）は、入力フォームを表示するのだ
    html := fmt.Sprintf(`
      <!DOCTYPE html>
      <html>
      <head><meta charset="utf-8"><title>テキスト連番保存</title></head>
      <body>
      <h2>メモ</h2>
      %s
      <hr>
      <h2>送信</h2>
      <p>クリップボードの画像はその辺でペーストすれば自動的に送信</p>
      <form id="memoForm" method="POST" action="/" enctype="multipart/form-data">
      <textarea id="memoText" name="text" rows="3" cols="40" placeholder="文字入力"></textarea>
      <br>
      <input type="file" name="image" accept="image/*"></input>
      <button type="submit" >実行</button>
      <script>
      // text area は enter で実行
      const memoText = document.getElementById('memoText');
      memoText.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
          e.preventDefault();
          document.getElementById('memoForm').submit();
        }
      });

      // 画面のどこでもペーストされたら発動するリスナーなのだ
      window.addEventListener('paste', async (e) => {
      // クリップボードの中身からファイル（画像など）を探すのだ
        const items = e.clipboardData.items;
        for (let item of items) {
          if (item.type.indexOf('image') === 0) {
          // 画像データを発見！ファイルオブジェクトとして取り出すのだ
          const file = item.getAsFile();
          
          // サーバーに送るためのフォームデータを作るのだ
          const formData = new FormData();
          formData.append('image', file); // Go側の r.FormFile("image") と名前を合わせるのだ！
      
          try {
            // 非同期でGoサーバーにPOST送信するのだ
            const response = await fetch('/', {
              method: 'POST',
              body: formData
            });
          
            if (response.ok) {
              // 送信成功したらページをリロードして、画像が追加された状態にするのだ
              location.reload();
            } else {
              alert('画像のアップロードに失敗したのだ…');
            }
          } catch (err) {
           console.error('通信エラー:', err);
          }
                
        // 画像が見つかったら処理を抜けるのだ
                break;
          }
        }
      });
      </script>
      </body>
      </html>

    `, contentHtml.String() )

    fmt.Fprint(w, html)
  }))

  fmt.Printf("サーバーを起動.  http://localhost%s にアクセスよろ\n",cfg.Port)
  if err := http.ListenAndServe(cfg.Port, nil); err != nil {
    fmt.Println("エラーなのだ:", err)
  }
}

func loadConfig() {
  configFile := "config.json"
  file, err := os.ReadFile(configFile)

  // "a" の bcrypt のハッシュ値を取る
  hashed, _ := bcrypt.GenerateFromPassword([]byte("a"), bcrypt.DefaultCost)
  
  // 初回はデフォルト値を作成
  // 止めないなら、そのまま使う
  if err != nil {
    cfg = Config{
      UploadDir: "./uploads",
      AuthUser:  "a",
      AuthPass:  string(hashed),
      Port:      ":8080",
    }
    data, _ := json.MarshalIndent(cfg, "", "  ")
    _ = os.WriteFile(configFile, data, 0644)
    fmt.Println("config.json が無かったので新規作成")
    return
  }

  // 読み込んだ json を構造体にしたがって展開
  _ = json.Unmarshal(file, &cfg)
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

func renumberFiles(dir string) {
  files, err := os.ReadDir(dir)
  if err != nil {
    return
  }
  // 現在の名前順にソート
  sort.Slice(files, func(i, j int) bool {
    return files[i].Name() < files[j].Name()
  })

  num := 1
  for _, f := range files {
    if f.IsDir() {
      continue
    }
    oldName := f.Name()
    ext := filepath.Ext(oldName)
    newName := fmt.Sprintf("%04d%s", num, ext)
    if oldName != newName {
      oldPath := filepath.Join(dir, oldName)
      newPath := filepath.Join(dir, newName)
      _ = os.Rename(oldPath, newPath)
    }
    num++
  }
}

func basicAuth(next http.HandlerFunc) http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    user, pass, ok := r.BasicAuth()
    if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(authUser)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(authPass)) != 1 {
      w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
      http.Error(w, "Unauthorized", http.StatusUnauthorized)
      return
    }
    next(w, r)
  }
}

func linkifyURLs(text string) string {
  urlRegex := regexp.MustCompile(`https?://[^\s<]+`)
  return urlRegex.ReplaceAllStringFunc(text, func(url string) string {
    return fmt.Sprintf("<a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">%s</a>", url, url)
  })
}
