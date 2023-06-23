# The decky plugin module is located at decky-loader/plugin
# For easy intellisense checkout the decky-loader code one directory up
# or add the `decky-loader/plugin` path to `python.analysis.extraPaths` in `.vscode/settings.json`
import asyncio
import os
import decky_plugin
from http import server
import ssl
from ssl import SSLContext
import multiprocessing
import json
import time
import socket
from settings import SettingsManager  # type: ignore
from urllib import parse
settings = SettingsManager(name="settings", settings_directory=os.environ["DECKY_PLUGIN_SETTINGS_DIR"])
settings.read()

base_dir = decky_plugin.DECKY_PLUGIN_DIR
certs_folder = os.path.join(decky_plugin.DECKY_PLUGIN_DIR, 'certs')


class MyHandler(server.SimpleHTTPRequestHandler):
    web_static_path = os.path.join(decky_plugin.DECKY_PLUGIN_DIR, "web")
    share_folder = None
    value = None

    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=self.web_static_path, **kwargs)

    def do_GET(self):
        try:
            self.value[0] = time.time()
            if self.path.startswith("/api"):
                path = self.path.split("/api")[1]
                if path.startswith('/download'):
                    self.send_response(200)
                    file_path = os.path.join(self.share_folder, path.split('/download/')[1].lstrip('/'))
                    # decode encoded URI
                    file_path = parse.unquote(file_path)
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
                            except Exception as e:
                                decky_plugin.logger.error(f"[MyHandler.do_GET]: {e}")
                                continue
                        self.wfile.write(json.dumps(directory_list).encode())
            else:
                return server.SimpleHTTPRequestHandler.do_GET(self)
        except Exception as e:
            decky_plugin.logger.error(f"[MyHandler.do_GET]: {e}")
            raise e

def start_file_server(last_called, error_event, error_text):
    try:
        decky_plugin.logger.info(f"[start_file_server]: Serving on port {settings.getSetting('PORT', 8000)}")
        handler = MyHandler
        handler.share_folder = os.path.join(settings.getSetting('DIRECTORY', '/home/deck'))
        handler.value = last_called
        httpd = server.HTTPServer(('0.0.0.0', settings.getSetting('PORT', 8000)), handler)
        context = SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(os.path.join(certs_folder, 'deckyfileserver_cert.pem'),
                                os.path.join(certs_folder, 'deckyfileserver_key.pem'))
        httpd.socket = context.wrap_socket(httpd.socket, server_side=True)
        httpd.serve_forever()
    except Exception as e:
        error_event.set()
        error_text[0] = f"[start_file_server]: {e}"
        decky_plugin.logger.error(f"[start_file_server]: {e}")
        raise e


class Plugin:
    server_running = False
    web_process = None
    timeout_process = None
    last_called = multiprocessing.Manager().list([time.time()])
    _watchdog_task = None
    error = multiprocessing.Manager().list([None])

    @asyncio.coroutine
    async def watchdog(self):
        while True:
            try:
                if not self.web_process:
                    await asyncio.sleep(15)
                    continue
                if self.web_process.is_alive() and time.time() - self.last_called[0] > 60 and self.server_running:
                    decky_plugin.logger.info("[check_last_call_expired]: Idle time exceeded, stopping server")
                    await Plugin.set_server_running(self, False)
                await asyncio.sleep(15)
            except Exception as e:
                decky_plugin.logger.error(f"[check_last_call_expired]: {e}")
                raise e

    async def set_server_running(self, enable: bool):
        try:
            if enable == self.server_running:
                return enable
            if enable:
                error_event = multiprocessing.Event()
                error_text = multiprocessing.Manager().list([None])
                decky_plugin.logger.info("[set_server_running] Starting web service...")
                self.last_called[0] = time.time()
                self.web_process = multiprocessing.Process(target=start_file_server, args=(self.last_called, error_event, error_text))
                self.web_process.daemon = True
                self.web_process.start()
                error_event.wait(timeout=2)
                await Plugin.set_error(self, None)
                if error_event.is_set():
                    await Plugin.set_error(self, error_text[0])
                    error_event.clear()
                    return await Plugin.set_server_running(self, False)
                else:
                    decky_plugin.logger.info("[set_server_running] Web service started")
                    self.server_running = True
            else:
                if self.web_process:
                    self.web_process.kill()
                decky_plugin.logger.info("[set_server_running] Stopping web service")
                self.server_running = False
            return enable
        except Exception as e:
            decky_plugin.logger.error(f"[set_server_running]: {e}")
            Plugin.set_server_running(self, False)
            raise e

    async def get_server_running(self):
        return self.server_running

    async def get_directory(self):
        return settings.getSetting('DIRECTORY', '')

    async def set_directory(self, directory: str):
        settings.setSetting('DIRECTORY', directory)
        settings.commit()
        return directory

    async def get_port(self):
        return settings.getSetting('PORT', '')

    async def set_port(self, port: int):
        settings.setSetting('PORT', int(port))
        settings.commit()
        return port

    async def get_error(self):
        return self.error[0]

    async def set_error(self, error: str | None):
        decky_plugin.logger.info(f"[set_error]: {error}")
        self.error[0] = error

    async def get_accepted_warning(self):
        return settings.getSetting("ACCEPTED_WARNING", False)

    async def accept_warning(self):
        decky_plugin.logger.info(f"[accept_warning]")
        settings.setSetting("ACCEPTED_WARNING", True)

    async def get_ip_address(self):
        return socket.gethostbyname(socket.gethostname())

    async def get_status(self):
        return {
            'server_running': await Plugin.get_server_running(self),
            'directory': await Plugin.get_directory(self),
            'port': await Plugin.get_port(self),
            'ip_address': await Plugin.get_ip_address(self),
            'error': await Plugin.get_error(self),
            'accepted_warning': await Plugin.get_accepted_warning(self),
        }

    async def set_status(self, status):
        try:
            self.error[0] = None
            if 'directory' in status:
                await Plugin.set_directory(self, status['directory'])
            if 'port' in status:
                await Plugin.set_port(self, status['port'])
            if 'server_running' in status:
                await Plugin.set_server_running(self, status['server_running'])
            return await Plugin.get_status(self)
        except Exception as e:
            decky_plugin.logger.error(f"[set_status]: {e}")
            raise e

    async def _main(self):
        try:
            if settings.getSetting('DIRECTORY', '') == '':
                settings.setSetting('DIRECTORY', '/home/deck')
                settings.commit()
            if settings.getSetting('PORT', '') == '':
                settings.setSetting('PORT', 8000)
                settings.commit()
            decky_plugin.logger.setLevel(20)

            loop = asyncio.get_event_loop()
            self._watchdog_task = loop.create_task(Plugin.watchdog(self))
            return
        except Exception as e:
            decky_plugin.logger.error(f"[_main]: {e}")
            raise

    # Function called first during the unload process, utilize this to handle your plugin being removed
    async def _unload(self):
        decky_plugin.logger.info("[_unload] Unloading plugin")
        await Plugin.set_server_running(self, False)
        self._watchdog_task.cancel()
        pass
