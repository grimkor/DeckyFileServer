import { useEffect, useState } from "react";
import {
  ButtonItem,
  Field,
  PanelSection,
  PanelSectionRow,
  showModal,
  ToggleField,
} from "@decky/ui";
import { callable } from "@decky/api";
import { State } from "../types";
import { WarningModal } from "./WarningModal";
import { SettingsPage } from "./SettingsPage";

const get_status = callable<[], State>("get_status");
const set_status = callable<[Partial<State>], State>("set_status");
const accept_warning = callable<[], void>("accept_warning");

export function Content() {
  const [state, setState] = useState<State>({
    server_running: false,
    directory: "/home/deck",
    port: 8000,
    timeout: 1,
    allow_uploads: false,
    ip_address: "127.0.0.1",
    accepted_warning: false,
    history: [],
    disable_thumbnails: false,
  });

  const getServerStatus = async () => {
    const initialState = await get_status();
    if (initialState) {
      setState(initialState);
    }
  };

  const setServerStatus = async (status: Partial<State>) => {
    const result = await set_status(status);
    if (result) {
      setState((prevState) => ({ ...prevState, ...result }));
    } else {
      await getServerStatus();
    }
  };

  useEffect(() => {
    getServerStatus();
    const timer = setInterval(getServerStatus, 5000);
    return () => clearInterval(timer);
  }, []);

  const onToggleEnableServer = async (checked: boolean) => {
    setState({ ...state, server_running: checked });
    if (state.accepted_warning) {
      await setServerStatus({ server_running: checked });
      return;
    }
    const onCancel = async () => {
      await setServerStatus({ server_running: false });
    };
    const onConfirm = async () => {
      await accept_warning();
      return setServerStatus({ server_running: checked });
    };
    if (state.accepted_warning) {
      await setServerStatus({ server_running: checked });
    } else {
      showModal(
        <WarningModal onCancel={onCancel} onConfirm={onConfirm} />,
        window,
      );
    }
  };

  const handleModalSubmit = async (
    port: number,
    timeout: number,
    directory: string,
    allow_uploads: boolean,
    disable_thumbnails: boolean,
  ) => {
    setServerStatus({
      port: Number(port),
      timeout: Number(timeout),
      directory,
      allow_uploads,
      disable_thumbnails,
    });
  };

  return (
    <PanelSection>
      <PanelSectionRow>
        <ToggleField
          checked={state.server_running}
          onChange={onToggleEnableServer}
          label="Enable Server"
        />
        {state.error ? <div>{state.error}</div> : null}
      </PanelSectionRow>
      <PanelSectionRow>
        <ButtonItem
          layout="below"
          disabled={state.server_running}
          onClick={() =>
            showModal(
              <SettingsPage
                port={state.port}
                timeout={state.timeout}
                directory={state.directory}
                history={state.history}
                allow_uploads={state.allow_uploads}
                disable_thumbnails={state.disable_thumbnails}
                handleSubmit={handleModalSubmit}
              />,
              window,
            )
          }
        >
          Settings
        </ButtonItem>
      </PanelSectionRow>
      <PanelSectionRow>
        <Field
          inlineWrap="shift-children-below"
          label="Server Address"
          bottomSeparator="none"
        >
          http{state.allow_uploads ? "s" : ""}://steamdeck:{state.port}
        </Field>
        <Field inlineWrap="shift-children-below">
          http{state.allow_uploads ? "s" : ""}://{state.ip_address}:{state.port}
        </Field>
        <Field
          inlineWrap="shift-children-below"
          label="Directory"
          bottomSeparator="none"
        >
          {state.directory}
        </Field>
      </PanelSectionRow>
    </PanelSection>
  );
}
