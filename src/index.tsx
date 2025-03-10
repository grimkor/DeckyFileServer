import React, {ChangeEvent, useEffect, useRef, useState, VFC} from 'react';
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
    Focusable,
    Toggle,
    DialogSubHeader,
} from "decky-frontend-lib";
import {FaServer, FaHistory} from "react-icons/fa";
import {IoMdAlert} from "react-icons/io";


interface State {
    server_running: boolean,
    directory: string,
    port: number,
    allow_uploads: boolean,
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
        allow_uploads: true,
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

    const handleModalSubmit = async (port: number, directory: string, allow_uploads: boolean) => {
        setServerStatus({
            port: Number(port),
            directory,
            allow_uploads,
        })
    };
    return (
        <PanelSection>
            <PanelSectionRow>
                <ToggleField checked={state.server_running} onChange={onToggleEnableServer} label="Enable Server"/>
                {state.error ? <div>{state.error}</div> : null}
            </PanelSectionRow>
            <PanelSectionRow>
                <ButtonItem
                    layout='below'
                    disabled={state.server_running}
                    onClick={() =>
                        showModal(<SettingsPage
                            port={state.port}
                            directory={state.directory}
                            history={state.history}
                            allow_uploads={state.allow_uploads}
                            serverAPI={serverAPI}
                            handleSubmit={handleModalSubmit}
                        />, window)}
                >
                    Settings
                </ButtonItem>
            </PanelSectionRow>
            <PanelSectionRow>
                <Field
                    inlineWrap="shift-children-below"
                    label="Server Address"
                    bottomSeparator='none'
                >
                    http{state.allow_uploads ? 's' : ''}://steamdeck:{state.port}
                </Field>
                <Field inlineWrap="shift-children-below">
                    http{state.allow_uploads ? 's' : ''}://{state.ip_address}:{state.port}
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
    allow_uploads: boolean;
    serverAPI: ServerAPI
    handleSubmit: (port: number, destination: string, https: boolean) => Promise<void>;
}> = ({
        closeModal,
        port,
        directory,
        serverAPI,
        history,
        allow_uploads,
        handleSubmit
      }) => {
    const [form, setForm] = useState({
        port,
        directory,
        allow_uploads: allow_uploads,
    });
    const [historySelection, setHistory] = useState("none");
    const [showPortError, setShowPortError] = useState(false);
    const ref = useRef<HTMLDivElement>(null);

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
    const handleDestinationClick = async (e: React.MouseEvent<HTMLElement>) => {
        e.stopPropagation();
        e.preventDefault();
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
            handleSubmit(form.port, form.directory, form.allow_uploads);
            closeModal?.();
        } else {
            setShowPortError(true);
        }
    };

    // TODO: USE FILE PICKER AND MAKE DIRECTORY TO SHARE FULL WIDTH
    return (
        <ModalRoot onCancel={handleClose}>
            <DialogBody style={{
                display: 'flex',
                flexDirection: 'column',
                height: '100%',
            }}>
                <DialogSubHeader>Directory to share</DialogSubHeader>
                <Focusable flow-children='right' style={{ display: 'flex', flex: 1, gap: 8 }}>
                    <DialogButton 
                        // @ts-ignore
                        onClick={handleDestinationClick}
                        style={{ flex: 1, textAlign: "left" }}
                    >
                        {form.directory}
                    </DialogButton>
                    <DialogButton
                        style={{
                            minWidth: 'fit-content',
                            width: 0,
                            padding: '20px',
                            display: 'flex',
                            justifyContent: 'center',
                            alignItems: 'center'
                        }}
                        onPointerDown={() => {
                            const container = document.getElementById("TESTING123");
                            console.log("pointerDown",container);
                        }}
                        onMouseDown={() => {
                            const container = document.getElementById("TESTING123");
                            console.log("pointerDown",container);
                        }}
                        onTouchStart={() => {
                            const container = document.getElementById("TESTING123");
                            console.log("touchStart",container);
                        }}
                        onClick={() => {
                            if (ref?.current) {
                                ref.current.getElementsByTagName('button')[0]?.click();
                            }
                        }}
                    >
                        <FaHistory fontSize={20} />
                    </DialogButton>
                    <div ref={ref} style={{display: 'none'}} id="TESTING123">
                        <DropdownItem
                            selectedOption={historySelection}
                            label={undefined}
                            strDefaultLabel={undefined}
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
                            bottomSeparator="none"

                        />
                    </div>
                </Focusable>
                <DialogSubHeader>Server</DialogSubHeader>
                <Field label="Port" icon={showPortError ? <IoMdAlert size={20} color="red"/> : null} bottomSeparator='none'>
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
                <Field label="Allow Uploads" bottomSeparator='none'>
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
