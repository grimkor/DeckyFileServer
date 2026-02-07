import asyncio
import os
from typing import List, Union
import decky # type: ignore
import socket
from settings import SettingsManager # type: ignore
import subprocess
from subprocess import PIPE

settings = SettingsManager(
    name="settings",
    settings_directory=os.environ["DECKY_PLUGIN_SETTINGS_DIR"]
)
settings.read()


def is_port_in_use(port: str) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        return s.connect_ex(("localhost", port)) == 0


class Plugin:
    backend = None
    server_running = False
    _watchdog_task = None
    error: Union[str, None] = None

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
                decky.logger.error(f"[check_last_call_expired]: {e}")
                raise e

    async def set_server_running(self, enable: bool):
        try:
            self.error = None
            if enable == self.server_running:
                decky.logger.info("[set_server_running] Server is already running, returning.")
                return True
            if enable:
                if is_port_in_use(await Plugin.get_port(self)):
                    Plugin.set_error(self, "Port is in use, select a different port.")
                    return enable
                decky.logger.info("[set_server_running] Starting web service...")
                decky.logger.info(f"[Plugin.get_directory]: {await Plugin.get_directory(self)}")
                self.backend = subprocess.Popen(
                    [
                        f"{decky.DECKY_PLUGIN_DIR}/bin/backend",
                        "-f",
                        await Plugin.get_directory(self),
                        "-p",
                        str(settings.getSetting("PORT")),
                        "-t",
                        str(await Plugin.get_timeout(self) * 60),
                        ("", "-uploads")[await Plugin.get_uploads_enabled(self)],
                        ("", "-disablethumbnails")[await Plugin.get_disable_thumbnails(self)]
                    ],
                    stdout=PIPE,
                    stderr=subprocess.STDOUT,
                )
                self.server_running = True
                decky.logger.info("[set_server_running] Web service started")
                await Plugin.set_history(self)
            else:
                if self.backend:
                    self.backend.terminate()
                decky.logger.info("[set_server_running] Stopping web service")
                self.server_running = False
                self.backend = None
            return enable
        except Exception as e:
            decky.logger.error(f"[set_server_running]: {e}")
            Plugin.set_error(self, str(e))

    async def get_server_running(self):
        return self.server_running

    async def get_history(self) -> List[str]:
        return settings.getSetting("HISTORY", [])

    async def set_history(self) -> List[str]:
        history = await Plugin.get_history(self)
        new_entry = await Plugin.get_directory(self)
        if new_entry == "":
            return history
        history = [h for h in history if h != new_entry]
        history.insert(0, new_entry)
        settings.setSetting("HISTORY", history[:10])
        settings.commit()
        return history

    async def get_directory(self) -> str:
        return settings.getSetting("DIRECTORY", "")

    async def set_directory(self, directory: str) -> str:
        settings.setSetting("DIRECTORY", directory)
        settings.commit()
        return directory

    async def get_port(self) -> str:
        return settings.getSetting("PORT", "")

    async def set_port(self, port: int) -> int:
        settings.setSetting("PORT", int(port))
        settings.commit()
        return port

    async def get_timeout(self) -> int:
        return int(settings.getSetting("TIMEOUT", "1"))

    async def set_timeout(self, timeout: int) -> int:
        settings.setSetting("TIMEOUT", int(timeout))
        settings.commit()
        return timeout

    async def get_uploads_enabled(self):
        return settings.getSetting("UPLOAD", False)

    async def set_uploads_enabled(self, enabled: bool):
        settings.setSetting("UPLOAD", enabled)
        settings.commit()
        return enabled

    async def get_disable_thumbnails(self):
        return settings.getSetting("DISABLE_THUMBNAILS", False)

    async def set_disable_thumbnails(self, enabled: bool):
        settings.setSetting("DISABLE_THUMBNAILS", enabled)
        settings.commit()
        return enabled

    async def get_error(self) -> Union[None, str]:
        return self.error

    def set_error(self, error: str):
        self.error = error

    async def get_accepted_warning(self):
        return settings.getSetting("ACCEPTED_WARNING", False)

    async def accept_warning(self):
        decky.logger.info("[accept_warning]")
        settings.setSetting("ACCEPTED_WARNING", True)

    async def get_ip_address(self):
        return socket.gethostbyname(socket.gethostname())

    async def get_status(self):
        return {
            "server_running": await Plugin.get_server_running(self),
            "directory": await Plugin.get_directory(self),
            "port": await Plugin.get_port(self),
            "timeout": await Plugin.get_timeout(self),
            "ip_address": await Plugin.get_ip_address(self),
            "accepted_warning": await Plugin.get_accepted_warning(self),
            "error": await Plugin.get_error(self),
            "history": await Plugin.get_history(self),
            "allow_uploads": await Plugin.get_uploads_enabled(self),
            "disable_thumbnails": await Plugin.get_disable_thumbnails(self)
        }

    async def set_status(self, status):
        try:
            if "directory" in status:
                await Plugin.set_directory(self, status["directory"])
            if "port" in status:
                await Plugin.set_port(self, status["port"])
            if "timeout" in status:
                await Plugin.set_timeout(self, status["timeout"])
            if "allow_uploads" in status:
                await Plugin.set_uploads_enabled(self, status["allow_uploads"])
            if "server_running" in status:
                await Plugin.set_server_running(self, status["server_running"])
            if "disable_thumbnails" in status:
                await Plugin.set_disable_thumbnails(self, status["disable_thumbnails"])
            return await Plugin.get_status(self)
        except Exception as e:
            decky.logger.error(f"[set_status]: {e}")
            raise e

    async def _main(self):
        try:
            if settings.getSetting("DIRECTORY", "") == "":
                settings.setSetting("DIRECTORY", decky.HOME)
                settings.commit()
            if settings.getSetting("PORT", "") == "":
                settings.setSetting("PORT", 8000)
                settings.commit()
            decky.logger.info("Started DeckyFileServer")
            loop = asyncio.get_event_loop()
            self._watchdog_task = loop.create_task(Plugin.watchdog(self))
        except Exception as e:
            decky.logger.error(f"[_main]: {e}")
            raise

    # Function called first during the unload process, utilize this to handle your plugin being removed
    async def _unload(self):
        if self.backend:
            self.backend.terminate()
        decky.logger.info("[_unload] Unloading plugin")
        await Plugin.set_server_running(self, False)
        if self._watchdog_task:
            self._watchdog_task.cancel()
        pass
