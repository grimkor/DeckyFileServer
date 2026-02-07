import React, { ChangeEvent, useEffect, useRef, useState, VFC } from "react";
import {
  DialogBody,
  DialogButton,
  DialogSubHeader,
  DropdownItem,
  Field,
  Focusable,
  ModalRoot,
  TextField,
  Toggle,
} from "@decky/ui";
import { FileSelectionType, openFilePicker } from "@decky/api";
import { FaHistory } from "react-icons/fa";
import { IoMdAlert } from "react-icons/io";
import { TIMEOUT_OPTIONS } from "../constants";

export const SettingsPage: VFC<{
  closeModal?: () => void;
  port: number;
  timeout: number;
  directory: string;
  history: string[];
  allow_uploads: boolean;
  disable_thumbnails: boolean;
  handleSubmit: (
    port: number,
    timeout: number,
    destination: string,
    allow_uploads: boolean,
    disable_thumbnails: boolean,
  ) => Promise<void>;
}> = ({
  closeModal,
  port,
  timeout,
  directory,
  history,
  allow_uploads,
  disable_thumbnails,
  handleSubmit,
}) => {
  const [form, setForm] = useState({
    port,
    timeout,
    directory,
    allow_uploads,
    disable_thumbnails,
  });
  const [historySelection, setHistory] = useState("none");
  const [showPortError, setShowPortError] = useState(false);
  const [showCustomTimeout, setShowCustomTimeout] = useState(
    !TIMEOUT_OPTIONS.includes(timeout),
  );
  const [customTimeout, setCustomTimeout] = useState(timeout);
  const ref = useRef<HTMLDivElement>(null);

  // the dropdown element is uncontrolled, force it back on change
  useEffect(() => {
    if (historySelection === "") {
      setHistory("none");
    }
  }, [historySelection]);

  const handleValueChange =
    (key: string) => (e: ChangeEvent<HTMLInputElement>) => {
      if (key === "port" && isNaN(parseInt(e.currentTarget.value))) {
        return;
      }
      if (key === "port") {
        setShowPortError(Number(parseInt(e.currentTarget.value)) < 1024);
      }
      setForm({
        ...form,
        [key]: parseInt(e.currentTarget.value),
      });
    };
  const handleDestinationClick = async (e: React.MouseEvent<HTMLElement>) => {
    e.stopPropagation();
    e.preventDefault();
    const file = await openFilePicker(
      FileSelectionType.FOLDER,
      directory,
      false,
    );
    if (file.path) {
      setForm({
        ...form,
        directory: file.path,
      });
    }
  };
  const handleClose = async () => {
    // check port is a number between 1024-65535 before closing
    if (Number(form.port) >= 1024 && Number(form.port) <= 65535) {
      await handleSubmit(
        form.port,
        form.timeout,
        form.directory,
        form.allow_uploads,
        form.disable_thumbnails,
      );
      closeModal?.();
    } else {
      setShowPortError(true);
    }
  };

  return (
    <ModalRoot onCancel={handleClose}>
      <DialogBody
        style={{
          display: "flex",
          flexDirection: "column",
          height: "100%",
        }}
      >
        <DialogSubHeader>Directory to share</DialogSubHeader>
        <Focusable
          flow-children="right"
          style={{ display: "flex", flex: 1, gap: 8 }}
        >
          <DialogButton
            // @ts-ignore
            onClick={handleDestinationClick}
            style={{ flex: 1, textAlign: "left" }}
          >
            {form.directory}
          </DialogButton>
          <DialogButton
            style={{
              minWidth: "fit-content",
              width: 0,
              padding: "20px",
              display: "flex",
              justifyContent: "center",
              alignItems: "center",
            }}
            onClick={() => {
              if (ref?.current) {
                ref.current.getElementsByTagName("button")[0]?.click();
              }
            }}
          >
            <FaHistory fontSize={20} />
          </DialogButton>
          <div ref={ref} style={{ display: "none" }}>
            <DropdownItem
              selectedOption={historySelection}
              label={undefined}
              strDefaultLabel={undefined}
              onChange={(sel) => {
                if (sel.data === "none") {
                  return;
                }
                setForm({
                  ...form,
                  directory: sel.data,
                });
                setHistory("");
              }}
              rgOptions={history.map((h) => ({ label: h, data: h }))}
              bottomSeparator="none"
            />
          </div>
        </Focusable>
        <DialogSubHeader>Server</DialogSubHeader>
        <Field
          label="Port"
          icon={showPortError ? <IoMdAlert size={20} color="red" /> : null}
          bottomSeparator="none"
        >
          <TextField
            description="Must be between 1024 and 65535"
            style={{
              boxSizing: "border-box",
              width: 160,
              height: 40,
              border: showPortError ? "1px red solid" : undefined,
            }}
            value={String(form.port)}
            defaultValue={form.port}
            onChange={handleValueChange("port")}
          />
        </Field>
        <Field label="Server Timeout (Minutes)" bottomSeparator="none">
          <DropdownItem
            selectedOption={showCustomTimeout ? -1 : form.timeout}
            label={undefined}
            strDefaultLabel={undefined}
            onChange={(sel) => {
              setShowCustomTimeout(sel.data === -1);
              setForm({
                ...form,
                timeout: sel.data === -1 ? customTimeout : sel.data,
              });
            }}
            rgOptions={[
              ...TIMEOUT_OPTIONS.map((x) => ({ label: x, data: x })),
              { label: "Custom", data: -1 },
            ]}
            bottomSeparator="none"
          />
        </Field>
        {showCustomTimeout ? (
          <Field label="Custom Timeout (Minutes)" bottomSeparator="none">
            <TextField
              style={{ boxSizing: "border-box", width: 100, height: 40 }}
              value={String(customTimeout)}
              defaultValue={customTimeout}
              disabled={!showCustomTimeout}
              onChange={(e) => {
                const value = Number(e.currentTarget.value);
                if (isNaN(value)) return;
                setCustomTimeout(value);
                handleValueChange("timeout")(e);
              }}
            />
          </Field>
        ) : null}
        <Field label="Allow Uploads" bottomSeparator="none">
          <Toggle
            value={form.allow_uploads}
            onChange={(value) =>
              setForm({
                ...form,
                allow_uploads: value,
              })
            }
          />
        </Field>
        <Field label="Disable Thumbnails" bottomSeparator="none">
          <Toggle
            value={form.disable_thumbnails}
            onChange={(value) =>
              setForm({
                ...form,
                disable_thumbnails: value,
              })
            }
          />
        </Field>
      </DialogBody>
    </ModalRoot>
  );
};
