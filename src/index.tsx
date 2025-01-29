import React, {ChangeEvent, useEffect, useState, VFC} from 'react';
import {
    ButtonItem,
    definePlugin,
    DialogBody,
    DialogFooter,
    DialogButton,
    DialogHeader,
    Field,
    ModalRoot,
    PanelSection,
    PanelSectionRow,
    ServerAPI,
    showModal,
    staticClasses,
    TextField,
    ToggleField,
    DropdownItem,
} from "decky-frontend-lib";
import {FaServer} from "react-icons/fa";
import {IoMdAlert} from "react-icons/io";


interface State {
    server_running: boolean,
    directory: string,
    port: number,
    https: boolean,
    ip_address: string,
    error?: string
    accepted_warning: boolean;
    history: string[]
}

const Content: VFC<{
    serverAPI: ServerAPI
}> = ({serverAPI}) => {
    const [state, setState] = useState<State>({
        server_running: false,
        directory: "/home/deck",
        port: 8000,
        https: true,
        ip_address: "127.0.0.1",
        accepted_warning: false,
        history: [],
    })


    const getServerStatus = async () => {
        const initialState = await serverAPI.callPluginMethod<undefined, State>("get_status", undefined);
        if (initialState.success) {
            setState(initialState.result);
        }
    }

    const setServerStatus = async (status: Partial<State>) => {
        const result = await serverAPI.callPluginMethod<{
            status: Partial<State>
        }, State>("set_status", {status});
        if (result.success) {
            setState(prevState => ({...prevState, ...result.result}));
        } else {
            await getServerStatus()
        }
    }

    useEffect(() => {
        getServerStatus()
        const timer = setInterval(getServerStatus, 5000);
        return () => clearInterval(timer);
    }, []);

    const onToggleEnableServer = async (checked: boolean) => {
        setState({...state, server_running: checked });
        if (state.accepted_warning) {
            await setServerStatus({server_running: checked});
            return;
        }
        const onCancel = async () => {
            await setServerStatus({server_running: false});
        }
        const onConfirm = async () => {
            serverAPI.callPluginMethod<undefined, undefined>('accept_warning', undefined);
            return setServerStatus({server_running: checked});
        }
        if (state.accepted_warning) {
            await setServerStatus({server_running: checked});
        } else {
            showModal(<WarningModal onCancel={onCancel} onConfirm={onConfirm} />, window)
        }
    };

    const handleModalSubmit = async (port: number, directory: string, https: boolean) => {
        setServerStatus({
            port: Number(port),
            directory,
            https,
        })
    };
    return (
        <>
            <PanelSection>
                <PanelSectionRow>
                    <ToggleField checked={state.server_running} onChange={onToggleEnableServer} label="Enable Server"/>
                    {state.error ? <div>{state.error}</div> : null}
                </PanelSectionRow>
            </PanelSection>
            <PanelSectionRow>
                <ButtonItem
                    disabled={state.server_running}
                    onClick={() =>
                        showModal(<SettingsPage
                            port={state.port}
                            directory={state.directory}
                            history={state.history}
                            https={state.https}
                            serverAPI={serverAPI}
                            handleSubmit={handleModalSubmit}
                        />, window)}
                >
                    Settings
                </ButtonItem>
            </PanelSectionRow>
            <PanelSection>
                <PanelSectionRow>
                    <Field
                        inlineWrap="shift-children-below"
                        label="Server Address"
                        bottomSeparator='none'
                    >
                        http{state.https ? 's' : ''}://steamdeck:{state.port}
                    </Field>
                    <Field inlineWrap="shift-children-below">
                        http{state.https ? 's' : ''}://{state.ip_address}:{state.port}
                    </Field>
                    <Field
                        inlineWrap="shift-children-below"
                        label="Directory"
                        bottomSeparator='none'
                    >
		    	{state.directory}
		    </Field>
                </PanelSectionRow>
            </PanelSection>
        </>
    )
};

const WarningModal = ({closeModal, onCancel, onConfirm}: {
    closeModal?: () => void;
    onCancel: () => void;
    onConfirm: () => Promise<void>;
    }) => {

    const handleCancel = () => {
        onCancel();
        closeModal?.();
    }

    const handleConfirm = async () => {
        await onConfirm();
        closeModal?.();
    }

    return (
        <ModalRoot closeModal={handleCancel}>
            <DialogHeader>Warning</DialogHeader>
            <DialogBody>
                <p>
                    Do not run this on an untrusted network as this will expose parts of the Steam Deck's file system to the network.
                </p>
                <p>
                    When accessing the URL you will receive a certificate security warning. This is because the plugin is using a self-signed certificate.
                </p>
            </DialogBody>
            <DialogFooter>
                    <DialogButton onClick={handleConfirm}>
                        Got it!
                    </DialogButton>
            </DialogFooter>
        </ModalRoot>
    );
}

const SettingsPage: VFC<{
    closeModal?: () => void;
    port: number;
    directory: string;
    history: string[];
    https: boolean;
    serverAPI: ServerAPI
    handleSubmit: (port: number, destination: string, https: boolean) => Promise<void>;
}> = ({
        closeModal,
        port,
        directory,
        serverAPI,
        history,
        https,
        handleSubmit
      }) => {
    const [form, setForm] = useState({
        port,
        directory,
        https,
    });
    const [historySelection, setHistory] = useState("none");
    const [showPortError, setShowPortError] = useState(false);

    // dropdown element is uncontrolled, force it back on change
    useEffect(() => {
        if (historySelection === "") {
            setHistory("none");            
        }
    }, [historySelection]);

    const handleValueChange = (key: string) => (e: ChangeEvent<HTMLInputElement>) => {
        if (key === 'port' && isNaN(parseInt(e.currentTarget.value))) {
            return;
        }
        setShowPortError(Number(parseInt(e.currentTarget.value)) < 1024);
        setForm({
            ...form,
            [key]: parseInt(e.currentTarget.value),
        });
    };
    const handleDestinationClick = async () => {
        const file = await serverAPI.openFilePicker(directory, false);
        if (file.path) {
            setForm({
                ...form,
                directory: file.path
            });
        }
    };
    const handleClose = () => {
        // check port is a number between 1024-65535 before closing
        if (Number(form.port) >= 1024 && Number(form.port) <= 65535) {
            handleSubmit(form.port, form.directory, form.https);
            closeModal?.();
        } else {
            setShowPortError(true);
        }
    };

    return (
        <ModalRoot onCancel={handleClose}>
            <DialogHeader>DeckyFileServer Settings</DialogHeader>
            <DialogBody>
                <Field label="Directory to Share">
                    <ButtonItem onClick={handleDestinationClick} bottomSeparator={"none"}>
                        Select Folder
                    </ButtonItem>
                </Field>
                <Field label="History">
                    <DropdownItem
                        selectedOption={historySelection}
                        strDefaultLabel={"Select from history..."}
                        onChange={sel => {
                            if (sel.data === "none") {
                                return;
                            }
                            setForm({
                                ...form,
                                directory: sel.data,
                            });
                            setHistory("");
                        }}
                        rgOptions={history.map(h => ({ label: h, data: h }))} 
                    />
                </Field>
                <Field>
                    {form.directory}
                </Field>
            </DialogBody>
            <DialogBody>
                <Field label="Port" icon={showPortError ? <IoMdAlert size={20} color="red"/> : null}>
                    <TextField
                        description="Must be between 1024 and 65535"
                        style={{
                            boxSizing: "border-box",
                            width: 160,
                            height: 40,
                            border: showPortError ? '1px red solid' : undefined
                        }}
                        value={String(form.port)}
                        defaultValue={form.port}
                        onChange={handleValueChange("port")}
                    />
                </Field>
            </DialogBody>
            <DialogBody>
                <Field label="Secure Connection (HTTPS)">
                    <DropdownItem
                        selectedOption={https ? "Yes" : "No"}
                        strDefaultLabel={"Yes"}
                        onChange={sel => {
                            setForm({
                                ...form,
                                https: sel.data === 'Yes',
                            });
                        }}
                        rgOptions={[{ label: "Yes", data: "Yes" }, { label: "No", data: "No" }]} 
                    />
                
                </Field>
            </DialogBody>
        </ModalRoot>
    );
};

export default definePlugin((serverApi: ServerAPI) => {
    return {
        title: <div className={staticClasses.Title}>DeckyFileServer</div>,
        content: <Content serverAPI={serverApi}/>,
        icon: <FaServer/>,
        onDismount() {
        },
    };
});
