import os
from http import server
import json
from urllib import parse

base_dir = os.path.dirname(os.path.abspath(__file__))


class MyHandler(server.SimpleHTTPRequestHandler):
    share_folder = "/home/decky"
    value = [0]
    web_static_path = os.path.join(base_dir, "www")

    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=self.web_static_path, **kwargs)

    def end_headers(self):
        self.send_header('Access-Control-Allow-Origin', '*')
        server.SimpleHTTPRequestHandler.end_headers(self)

    def do_GET(self):
        print(self.path)
        if self.path.startswith("/api"):
            path = self.path.split("/api")[1]
            if path.startswith('/download'):
                self.send_response(200)
                file_path_encoded = os.path.join(self.share_folder, path.split('/download/')[1].lstrip('/'))
                file_path = parse.unquote(file_path_encoded)
                print(file_path)
                if os.path.exists(file_path):
                    with open(file_path, 'rb') as f:
                        self.send_header('Content-type', 'application/octet-stream')
                        self.end_headers()
                        self.wfile.write(f.read())
                else:
                    self.send_header('Content-type', 'text/html')
                    self.end_headers()
                    self.wfile.write(b'File not found')
            if path.startswith('/browse'):
                requested_path = os.path.join(self.share_folder, path.replace('/browse', '').lstrip("/"))
                if os.path.isdir(os.path.join(self.share_folder, requested_path)):
                    self.send_response(200)
                    self.send_header('Content-type', 'application/json')
                    self.end_headers()
                    directory_list = {}
                    for file in os.listdir(os.path.join(self.share_folder, requested_path)):
                        try:
                            directory_list[file] = {
                                "isdir": os.path.isdir(os.path.join(self.share_folder, requested_path, file)),
                                "size": os.path.getsize(os.path.join(self.share_folder, requested_path, file)),
                                "modified": os.path.getmtime(os.path.join(self.share_folder, requested_path, file))
                            }
                        except Exception:
                            continue
                    self.wfile.write(json.dumps(directory_list).encode())
        else:
            return server.SimpleHTTPRequestHandler.do_GET(self)


handler = MyHandler
handler.share_folder = "/home/decky"
handler.value = []
http = server.HTTPServer(("", 9999), handler)



if __name__ == '__main__':
    http.serve_forever()
