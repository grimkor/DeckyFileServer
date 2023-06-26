# The decky plugin module is located at decky-loader/plugin
# For easy intellisense checkout the decky-loader code one directory up
# or add the `decky-loader/plugin` path to `python.analysis.extraPaths` in `.vscode/settings.json`
import asyncio
import os
import decky_plugin
import socket
from settings import SettingsManager  # type: ignore
import subprocess
from subprocess import PIPE

settings = SettingsManager(name="settings", settings_directory=os.environ["DECKY_PLUGIN_SETTINGS_DIR"])
settings.read()


def is_port_in_use(port: int) -> bool:
    import socket
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        return s.connect_ex(('localhost', port)) == 0


class Plugin:
    backend = None
    server_running = False
    _watchdog_task = None
    error = None

    @asyncio.coroutine
    async def watchdog(self):
        while True:
            try:
                if not self.backend:
                    await asyncio.sleep(1)
                    continue
                if self.backend.poll() is None:
                    await asyncio.sleep(1)
                    continue
                await Plugin.set_server_running(self, False)
                await asyncio.sleep(1)
            except Exception as e:
                decky_plugin.logger.error(f"[check_last_call_expired]: {e}")
                raise e

    async def set_server_running(self, enable: bool):
        try:
            self.error = None
            if enable == self.server_running:
                return True
            if enable:
                if is_port_in_use(await Plugin.get_port(self)):
                    Plugin.set_error(self, "Port is in use, select a different port.")
                    return enable
                decky_plugin.logger.info("[set_server_running] Starting web service...")
                decky_plugin.logger.info(f"[Plugin.get_directory]: {await Plugin.get_directory(self)}")
                self.backend = subprocess.Popen(
                    [
                        f"{decky_plugin.DECKY_PLUGIN_DIR}/bin/backend",
                        await Plugin.get_directory(self),
                        str(settings.getSetting("PORT")),
                        decky_plugin.DECKY_PLUGIN_DIR
                    ],
                    stdout=PIPE,
                    stderr=subprocess.STDOUT,
                )
                self.server_running = True
                decky_plugin.logger.info("[set_server_running] Web service started")
            else:
                if self.backend:
                    self.backend.terminate()
                decky_plugin.logger.info("[set_server_running] Stopping web service")
                self.server_running = False
            return enable
        except Exception as e:
            decky_plugin.logger.error(f"[set_server_running]: {e}")
            Plugin.set_error(self, str(e))

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
        return self.error

    def set_error(self, error):
        self.error = error

    async def get_accepted_warning(self):
        return settings.getSetting("ACCEPTED_WARNING", False)

    async def accept_warning(self):
        decky_plugin.logger.info("[accept_warning]")
        settings.setSetting("ACCEPTED_WARNING", True)

    async def get_ip_address(self):
        return socket.gethostbyname(socket.gethostname())

    async def get_status(self):
        return {
            'server_running': await Plugin.get_server_running(self),
            'directory': await Plugin.get_directory(self),
            'port': await Plugin.get_port(self),
            'ip_address': await Plugin.get_ip_address(self),
            'accepted_warning': await Plugin.get_accepted_warning(self),
            'error': await Plugin.get_error(self),
        }

    async def set_status(self, status):
        try:
            decky_plugin.logger.info(status)
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
                settings.setSetting('DIRECTORY', decky_plugin.HOME)
                settings.commit()
            if settings.getSetting('PORT', '') == '':
                settings.setSetting('PORT', 8000)
                settings.commit()
            decky_plugin.logger.info("Started DeckyFileServer")
            loop = asyncio.get_event_loop()
            self._watchdog_task = loop.create_task(Plugin.watchdog(self))
        except Exception as e:
            decky_plugin.logger.error(f"[_main]: {e}")
            raise

    # Function called first during the unload process, utilize this to handle your plugin being removed
    async def _unload(self):
        self.backend.terminate()
        decky_plugin.logger.info("[_unload] Unloading plugin")
        await Plugin.set_server_running(self, False)
        self._watchdog_task.cancel()
        pass
