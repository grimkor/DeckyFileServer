class Navigator {
    list = {};
    path = [];

    async init() {
        await this.getFolder();
        this.populateFileList();
    }
    addToPath(p) {
        this.path.push(p)
    }

    goBack() {
        this.path = this.path.slice(0, this.path.length -1);
        this.populateFileList();
    }

    getPath() {
        return this.path.join('/');
    }

    async getFolder(path=null) {
        if (path) {
            this.addToPath(path);
        }
        const response = await fetch(`/api/browse/${this.getPath()}`); 

        if (response.ok) {
            this.list = await response.json();
        }
    }

    downloadFile(file) {
        const a = document.createElement('a');
        a.href = '/api/download/' + this.getPath() + '/' + file;
        a.click();
    }

    populateFileList() {
        const fileListElement = document.getElementById("file-list");
        if (fileListElement) {
                const elBack = document.getElementById('back-button');
            if (this.path.length) {
                elBack.classList.remove('hidden')
            } else {
                elBack.classList.add('hidden')
            }
            while(fileListElement.firstChild != null) {
                fileListElement.removeChild(fileListElement.firstChild)
            }
            Object.entries(this.list)
                .filter(([name,]) => this.showHidden || name[0] !== ".")
                .map(([name,item]) => {
                const el = document.createElement('div')
                el.className = 'file-row'
                const icon = document.createElement('img');
                icon.className = "file-icon_img";
                icon.src = item.isdir ? "/folder.svg" : "/file.svg";
                el.appendChild(icon);
                const elDetails = document.createElement('div');
                elDetails.className = "file-details";
                elDetails.innerText = name;
                const elStats = document.createElement('div');
                elStats.className = "file-details_description";
                elStats.innerText = "205k, 13:37, 17 June 2023";
                elDetails.appendChild(elStats);
                el.appendChild(elDetails);
                el.onclick = () =>{ 
                    if (item.isdir) {
                        this.addToPath(name);
                        this.populateFileList();
                    } else {
                        this.downloadFile(name);
                    }
                }
                fileListElement.appendChild(el);
            })
        }
    }
    toggleShowHidden() {
        this.showHidden = !this.showHidden;    
        this.populateFileList();
    }
}

let Nav = new Navigator();
Nav.init();
function getDialog() {
    return document.getElementById('menu-dialog');
}

function showMenu() {
    const dialog = getDialog();
    dialog.showModal();
}

function closeMenu() {
    const dialog = getDialog();
    dialog.close();
}

//getDialog().addEventListener("click", closeMenu);

function handleHiddenClick() {
    Nav.toggleShowHidden();
    getDialog().close();
}
