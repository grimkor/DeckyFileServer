import {useCallback, useEffect, useState} from 'react'
import './App.css'
import folderLogo from '/folder.svg'
import fileLogo from '/file.svg'
import backLogo from '/back_outline.svg'
import menuLogo from '/menu.svg'

function formatSizeUnits(bytes: number) {
    if (bytes >= 1073741824) {
        return (bytes / 1073741824).toFixed(2) + " GB";
    } else if (bytes >= 1048576) {
        return (bytes / 1048576).toFixed(2) + " MB";
    } else if (bytes >= 1024) {
        return (bytes / 1024).toFixed(2) + " KB";
    } else if (bytes > 1) {
        return bytes + " bytes";
    } else if (bytes === 1) {
        return bytes + " byte";
    } else {
        return "0 bytes";
    }
}

const formatModifiedDate = (modified: number) => {
    const date = new Date(modified * 1000);

    return date.toDateString()
}

interface Item {
    name: string;
    isdir: boolean;
    size: number;
    modified: number;
}

type Directory = Item[];

function App() {
    const [path, setPath] = useState<string[]>([]);
    const [directory, setDirectory] = useState<Directory>([])
    const [showMenu, setShowMenu] = useState(false);
    const [showHiddenFiles, setShowHiddenFiles] = useState(false);
    const [sortAToZ, setSortAToZ] = useState(true);

    const handleBack = useCallback(() => {
        if (!path.length) {
            return;
        }
        return setPath(path.slice(0, -1))
    }, [path]);

    useEffect(() => {
        const getData = async () => {
            try {
                const response = await fetch(`/api/browse?path=${path.join('/')}`);
                if (response.ok) {
                    setDirectory(await response.json());
                } else {
                    handleBack();
                }
            } catch (e) {
                handleBack();
                console.error(e);
            } finally {
                scrollTo(0, 0);
            }
        }
        getData();
    }, [handleBack, path])


    const handleClick = (name: string, isDir: boolean) => () => {
        if (isDir) {
            setPath([...path, name]);
        } else {
            const a = document.createElement('a');
            a.href = `/api/download/${path.join('/')}/${name}`;
            a.download = name;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        }
    }

    const handleHiddenFiles = () => {
        setShowHiddenFiles(prev => !prev);
        setShowMenu(false);
    }

    const handleSortAToZ = () => {
        setSortAToZ(prev => !prev);
        setShowMenu(false);
    }

    return (
        <div className={"container"}>
            <div className={"menu"}>
                {path.length ? <div className={"back-button"} onClick={handleBack}>
                    <img src={backLogo} className={"file-icon_img"}/>
                </div> : <div/>}
                <div id={"menu-button"} className={"menu-button"} onClick={() => setShowMenu(true)}>
                    <img src={menuLogo} className={"file-icon_img"}/>
                </div>
                {showMenu ? (
                    <>
                        <div className={"menu-backdrop"} onClick={() => setShowMenu(false)}/>
                        <div className={"menu-popup"}>
                            <div className={"menu-list"}>
                                <div className={"menu-item"}
                                     onClick={handleHiddenFiles}>{showHiddenFiles ? "Hide" : "Show"} Hidden Files
                                </div>
                                <div className={"menu-item"} onClick={handleSortAToZ}>Sort Alphabetically
                                    ({sortAToZ ? "Z-A" : "A-Z"})
                                </div>
                            </div>
                        </div>
                    </>
                ) : null}
            </div>
            <div className={"file-list"}>
                {directory
                    .sort((a, b) => (a.name > b.name) === sortAToZ ? 1 : -1)
                    .filter((file) => file.name[0] !== '.' || showHiddenFiles)
                    .map((({isdir, modified, size, name}) =>
                            (
                                <div className={"file-row"} onClick={handleClick(name, isdir)}>
                                    {isdir ?
                                        <img src={folderLogo} className={"file-icon_img"}/> :
                                        <img src={fileLogo} className={"file-icon_img"}/>}
                                    <div className={"file-details"}>
                                        <div className={"file-details_name"}>
                                            {name}
                                        </div>
                                        <div className={"file-details_description"}>
                                            {isdir ? '' : formatSizeUnits(size) + ','} {formatModifiedDate(modified)}
                                        </div>
                                    </div>
                                </div>
                            )
                    ))}
            </div>
        </div>
    )
}

export default App
