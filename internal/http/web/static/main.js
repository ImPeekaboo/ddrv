const app = angular.module('application', []);

app.config(['$httpProvider', function ($httpProvider) {
    $httpProvider.interceptors.push(['$q', '$rootScope', '$location', function ($q, $rootScope, $location) {
        return {
            'responseError': function (rejection) {
                if (rejection.status === 401 && $location.path() !== '/api/check_token') {
                    $rootScope.$broadcast('loginRequired');
                }
                return $q.reject(rejection);
            }
        };
    }]);
}]);

app.run(['$rootScope', 'AuthService', '$location', function ($rootScope, AuthService, $location) {
    $rootScope.config = {}
    $rootScope.appCrashed = false
    AuthService.config().then((config) => {
        $rootScope.authenticated = false
        $rootScope.$apply(() => $rootScope.config = config.data);
        if (config.data.login && AuthService.getToken()) {
            AuthService.checkToken().then(() => {
                $rootScope.authenticated = true
            }).catch(() => {
                if (!config.data.anonymous) $rootScope.showLoginModal()
            })
        }
    })

    $rootScope.showLoginModal = function () {
        const loginDialog = document.getElementById("loginDialog")
        if (!loginDialog.open) {
            loginDialog.showModal()
        }
    }

    $rootScope.closeLoginModal = function () {
        const loginDialog = document.getElementById("loginDialog")
        if (loginDialog.open) {
            loginDialog.close()
        }
    }
    $rootScope.$on('loginRequired', $rootScope.showLoginModal.bind(this));
    $rootScope.baseURL = $location.protocol() + '://' + $location.host() + ':' + $location.port();
}]);

app.controller('authController', ['$scope', '$rootScope', 'AuthService', function ($scope, $rootScope, AuthService) {
    $scope.login = async function () {
        try {
            await AuthService.login($scope.username, $scope.password);
            $scope.closeLoginModal()
            $rootScope.$apply(() => {
                $rootScope.authenticated = true
            })
        } catch (err) {
            $scope.$apply(() => $scope.loginErrMessage = err.data.message);
        }
    }

    $scope.logout = function () {
        AuthService.logout();
    }
}])

app.controller('controller', ['$scope', 'FMService', '$interval', function ($scope, FMService, $interval) {

    // current directory
    $scope.directory = {files: []}

    $scope.selectAll = function (selectAllCheckbox) {
        $scope.directory.files.forEach(function (file) {
            file.selected = selectAllCheckbox;
        });
    };

    $scope.$watch('directory.files', updateButtons.bind(this), true);
    $scope.$watch('authenticated', function () {
        updateButtons()
        $scope.load()
    }, true);

    function updateButtons() {
        let selectedFiles = $scope.directory.files.filter(file => file.selected);
        if (!$scope.config.login || $scope.authenticated) {
            $scope.btnCreateFolder = selectedFiles.length === 0;
            $scope.btnUploadFile = selectedFiles.length === 0;
            $scope.btnCopyLink = selectedFiles.length === 1 && !selectedFiles[0].dir;
            $scope.btnRename = selectedFiles.length === 1;
            $scope.btnDelete = selectedFiles.length > 0;
        } else if ($scope.config.anonymous && !$scope.authenticated) {
            $scope.btnCreateFolder = false;
            $scope.btnUploadFile = false;
            $scope.btnCopyLink = selectedFiles.length === 1 && !selectedFiles[0].dir;
            $scope.btnRename = false;
            $scope.btnDelete = false;
        }
    }

    $scope.load = function (id) {
        FMService.getDir(id).then((directory) => {
            $scope.$apply(() => $scope.directory = directory)
        })
    }

    $scope.open = function (file) {
        const url = `${$scope.baseURL}/files/${file.id}/${file.name}`
        window.open(url, '_blank');
    }

    $scope.createFolder = async function () {
        try {
            await FMService.createDir({parent: $scope.directory.id, name: $scope.newFolderName})
            document.getElementById('createFolderDialog').close()
            $scope.load($scope.directory.id)
            $scope.newFolderName = ''
            $scope.createFolderErrorMessage = ''
        }catch (err) {
            $scope.$apply(() => $scope.createFolderErrorMessage = err.data.message)
        }
    }

    $scope.progressbars = [];

    $scope.progressCallback = function (fileName, progress) {
        let progressBar = $scope.progressbars.find(bar => bar.name === fileName);
        if (progressBar) {
            $scope.$apply(function () {
                progressBar.value = progress;
            });
        }
    }

    $scope.upload = function () {
        document.getElementById('fileUpload').click();
    }

    $scope.fileChanged = function (files) {
        angular.forEach(files, function (file) {
            $scope.progressbars.push({name: file.name, value: 0}); // Create new progress bar for each file
            FMService.createFile($scope.directory.id, file, $scope.progressCallback).then(() => {
                $scope.load($scope.directory.id);
                let progressBarIndex = $scope.progressbars.findIndex(bar => bar.name === file.name);
                if (progressBarIndex !== -1) {
                    $scope.progressbars.splice(progressBarIndex, 1);
                }
            }).catch(err => {
                $scope.progressCallback(file.name, `failed`)
            });
        });
    }

    $scope.progressCallback = function (fileName, progress) {
        let progressBar = $scope.progressbars.find(bar => bar.name === fileName);
        if (progressBar) {
            progressBar.value = progress;
        }
    }

    $scope.rename = async function () {
        try {
            const resource = $scope.directory.files.find(f => f.selected === true)
            if (resource) {
                if (resource.dir) {
                    const directory = {name: $scope.newName, parent: resource.parent}
                    await FMService.updateDir(resource.id, directory)
                } else {
                    const file = {name:$scope.newName, parent: resource.parent}
                    await FMService.updateFile($scope.directory.id, resource.id, file)
                }
                $scope.load($scope.directory.id)
                document.getElementById("renameDialog").close()
            }
            $scope.newName = ''
            $scope.renameErrorMessage = ''
        }catch (err) {
            $scope.$apply(() => $scope.renameErrorMessage = err.data.message)
        }
    }

    $scope.copyLink = function () {
        const file = $scope.directory.files.find(f => f.selected === true)
        copyTextToClipboard(`${$scope.baseURL}/files/${file.id}`)
    }

    $scope.delete = async function () {
        const files = $scope.directory.files.filter(f => f.selected === true)
        for (const file of files) {
            await FMService.deleteDir(file.id)
        }
        await $scope.load($scope.directory.id)
    }
}]);

app.service('AuthService', ['$http', '$window', function ($http, $window) {
    const token = $window.localStorage.getItem('auth_token');
    if (token) {
        $http.defaults.headers.common.Authorization = 'Bearer ' + token;
    }
    return {
        config: async function () {
            try {
                const response = await $http.get('/api/config');
                return response.data;
            } catch (error) {
                throw error
            }
        },
        login: async function (username, password) {
            const response = await $http.post('/api/user/login', {
                username: username,
                password: password
            });
            const {data} = response.data
            $window.localStorage.setItem('auth_token', data);
            $http.defaults.headers.common.Authorization = 'Bearer ' + data;
        },
        logout: function () {
            $window.localStorage.removeItem('auth_token');
            location.reload();
        },
        getToken: function () {
            return $window.localStorage.getItem('auth_token');
        },
        checkToken: function () {
            return $http.get('/api/check_token')
        }
    };
}]);

app.service('FMService', ['$http', function ($http) {
    return {
        createDir: function (directory) {
            return $http.post('/api/directories/', directory);
        },
        getDir: async function (id) {
            const endpoint = id ? '/api/directories/' + id : '/api/directories'
            const {data: {data: dir}} = await $http.get(endpoint)
            if (!dir.files) dir.files = []
            dir.files = dir.files.map(f => {
                return {...f, size: f.dir ? 'folder' : humanReadableSize(f.size), selected: false}
            })
            return dir
        },
        updateDir: function (id, directory) {
            return $http.put('/api/directories/' + id, directory);
        },
        deleteDir: function (id) {
            return $http.delete('/api/directories/' + id);
        },
        createFile: function (dirId, file, progressCallback) {
            const formData = new FormData();
            formData.append('file', file);

            return $http({
                method: 'POST',
                url: '/api/directories/' + dirId + '/files',
                data: formData,
                headers: {'Content-Type': undefined}, // Let browser set the content-type
                uploadEventHandlers: {
                    progress: function (e) {
                        if (e.lengthComputable) {
                            let progress = (e.loaded / e.total * 100).toFixed(2);
                            progressCallback(file.name, progress);
                        }
                    }
                }
            });
        },
        updateFile: function (dirId, id, file) {
            return $http.put('/api/directories/' + dirId + '/files/' + id, file);
        },
        deleteFile: function (dirId, id) {
            return $http.delete('/api/directories/' + dirId + '/files/' + id);
        },
    };
}]);

function humanReadableSize(bytes, si = false, dp = 1) {
    const thresh = si ? 1000 : 1024

    if (Math.abs(bytes) < thresh) {
        return `${bytes} B`
    }

    const units = si
        ? ['kB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
        : ['KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB', 'ZiB', 'YiB']
    let u = -1
    const r = 10 ** dp

    do {
        bytes /= thresh
        u += 1
    } while (Math.round(Math.abs(bytes) * r) / r >= thresh && u < units.length - 1)

    return `${bytes.toFixed(dp)} ${units[u]}`
}

function fallbackCopyTextToClipboard(text) {
    const textArea = document.createElement('textarea');
    textArea.value = text;

    // Avoid scrolling to bottom
    textArea.style.top = '0';
    textArea.style.left = '0';
    textArea.style.position = 'fixed';

    document.body.appendChild(textArea);
    textArea.focus();
    textArea.setSelectionRange(0, text.length); // Set selection range for mobile devices
    document.execCommand('copy');
    document.body.removeChild(textArea);
}

function copyTextToClipboard(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text)
            .catch(() => fallbackCopyTextToClipboard(text));
    } else {
        fallbackCopyTextToClipboard(text);
    }
}

function triggerDownload(url, name) {
    const link = document.createElement('a');
    link.href = url;
    link.download = name;
    link.click();
}