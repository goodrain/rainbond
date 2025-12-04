/**
 * 分片上传客户端示例
 *
 * 使用方法：
 * const uploader = new ChunkUploader(file, eventID);
 * await uploader.upload((progress) => {
 *   console.log(`上传进度: ${progress}%`);
 * });
 */

class ChunkUploader {
    constructor(file, eventID, options = {}) {
        this.file = file;
        this.eventID = eventID;
        this.chunkSize = options.chunkSize || 5 * 1024 * 1024; // 默认5MB
        this.concurrency = options.concurrency || 3; // 并发上传数
        this.baseURL = options.baseURL || '/package_build';
        this.sessionID = null;
        this.totalChunks = Math.ceil(file.size / this.chunkSize);
    }

    /**
     * 初始化上传会话
     * @returns {Promise<Object>} 会话信息
     */
    async init() {
        const response = await fetch(`${this.baseURL}/component/events/${this.eventID}/upload/init`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                file_name: this.file.name,
                file_size: this.file.size,
                chunk_size: this.chunkSize,
            }),
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(`初始化失败: ${error.msg || error.message}`);
        }

        const data = await response.json();
        this.sessionID = data.data.session_id;
        this.totalChunks = data.data.total_chunks;

        console.log(`上传会话已创建: ${this.sessionID}`);
        console.log(`总分片数: ${this.totalChunks}`);
        console.log(`已上传分片: ${data.data.uploaded_chunks.length}/${this.totalChunks}`);

        return data.data;
    }

    /**
     * 上传单个分片
     * @param {number} chunkIndex - 分片索引
     * @returns {Promise<Object>} 上传结果
     */
    async uploadChunk(chunkIndex) {
        const start = chunkIndex * this.chunkSize;
        const end = Math.min(start + this.chunkSize, this.file.size);
        const chunk = this.file.slice(start, end);

        const formData = new FormData();
        formData.append('session_id', this.sessionID);
        formData.append('chunk_index', chunkIndex.toString());
        formData.append('file', chunk, `chunk_${chunkIndex}`);

        const response = await fetch(`${this.baseURL}/component/events/${this.eventID}/upload/chunk`, {
            method: 'POST',
            body: formData,
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(`分片 ${chunkIndex} 上传失败: ${error.msg || error.message}`);
        }

        return await response.json();
    }

    /**
     * 完成上传，触发服务端合并分片
     * @returns {Promise<Object>} 完成结果
     */
    async complete() {
        const response = await fetch(`${this.baseURL}/component/events/${this.eventID}/upload/complete`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                session_id: this.sessionID,
            }),
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(`完成上传失败: ${error.msg || error.message}`);
        }

        return await response.json();
    }

    /**
     * 查询上传状态
     * @returns {Promise<Object>} 状态信息
     */
    async getStatus() {
        const response = await fetch(
            `${this.baseURL}/component/events/${this.eventID}/upload/status/${this.sessionID}`,
            { method: 'GET' }
        );

        if (!response.ok) {
            const error = await response.json();
            throw new Error(`获取状态失败: ${error.msg || error.message}`);
        }

        return await response.json();
    }

    /**
     * 取消上传
     * @returns {Promise<void>}
     */
    async cancel() {
        if (!this.sessionID) {
            return;
        }

        const response = await fetch(
            `${this.baseURL}/component/events/${this.eventID}/upload/${this.sessionID}`,
            { method: 'DELETE' }
        );

        if (!response.ok) {
            const error = await response.json();
            throw new Error(`取消上传失败: ${error.msg || error.message}`);
        }

        console.log('上传已取消');
    }

    /**
     * 上传所有分片（支持断点续传）
     * @param {Function} onProgress - 进度回调函数 (progress: number) => void
     * @returns {Promise<string>} 文件路径
     */
    async upload(onProgress) {
        try {
            // 1. 初始化会话
            const initData = await this.init();
            const uploadedChunks = new Set(initData.uploaded_chunks || []);

            // 2. 确定需要上传的分片
            const chunksToUpload = [];
            for (let i = 0; i < this.totalChunks; i++) {
                if (!uploadedChunks.has(i)) {
                    chunksToUpload.push(i);
                }
            }

            if (chunksToUpload.length === 0) {
                console.log('所有分片已上传，直接合并');
                const result = await this.complete();
                return result.data.file_path;
            }

            console.log(`需要上传 ${chunksToUpload.length} 个分片`);

            // 3. 并发上传分片
            let completedCount = uploadedChunks.size;
            for (let i = 0; i < chunksToUpload.length; i += this.concurrency) {
                const batch = chunksToUpload.slice(i, i + this.concurrency);

                await Promise.all(
                    batch.map(async (chunkIndex) => {
                        try {
                            await this.uploadChunk(chunkIndex);
                            completedCount++;

                            // 调用进度回调
                            if (onProgress) {
                                const progress = (completedCount / this.totalChunks) * 100;
                                onProgress(progress);
                            }

                            console.log(`分片 ${chunkIndex + 1}/${this.totalChunks} 上传成功`);
                        } catch (error) {
                            console.error(`分片 ${chunkIndex} 上传失败:`, error);
                            throw error;
                        }
                    })
                );
            }

            // 4. 完成上传
            console.log('所有分片上传完成，开始合并...');
            const result = await this.complete();
            console.log('文件合并成功:', result.data.file_path);

            return result.data.file_path;

        } catch (error) {
            console.error('上传失败:', error);
            throw error;
        }
    }

    /**
     * 断点续传（从上次中断的地方继续）
     * @param {Function} onProgress - 进度回调函数
     * @returns {Promise<string>} 文件路径
     */
    async resume(onProgress) {
        console.log('恢复上传...');
        return await this.upload(onProgress);
    }
}

// ============== 使用示例 ==============

/**
 * 示例1：基本使用
 */
async function example1() {
    const fileInput = document.getElementById('file-input');
    const file = fileInput.files[0];
    const eventID = 'your-event-id';

    const uploader = new ChunkUploader(file, eventID);

    try {
        const filePath = await uploader.upload((progress) => {
            console.log(`上传进度: ${progress.toFixed(2)}%`);
            // 更新进度条UI
            document.getElementById('progress-bar').style.width = `${progress}%`;
        });

        console.log('上传完成，文件路径:', filePath);
        alert('上传成功！');
    } catch (error) {
        console.error('上传失败:', error);
        alert('上传失败: ' + error.message);
    }
}

/**
 * 示例2：支持断点续传
 */
async function example2WithResume() {
    const file = document.getElementById('file-input').files[0];
    const eventID = 'your-event-id';

    const uploader = new ChunkUploader(file, eventID, {
        chunkSize: 10 * 1024 * 1024,  // 10MB分片
        concurrency: 5,                // 5个并发上传
    });

    try {
        const filePath = await uploader.upload((progress) => {
            document.getElementById('progress').textContent = `${progress.toFixed(1)}%`;
        });
        console.log('上传完成:', filePath);
    } catch (error) {
        console.error('上传出错，尝试断点续传...');

        // 等待1秒后重试
        await new Promise(resolve => setTimeout(resolve, 1000));

        try {
            const filePath = await uploader.resume((progress) => {
                document.getElementById('progress').textContent = `恢复上传: ${progress.toFixed(1)}%`;
            });
            console.log('断点续传成功:', filePath);
        } catch (resumeError) {
            console.error('断点续传也失败了:', resumeError);
            alert('上传失败，请稍后重试');
        }
    }
}

/**
 * 示例3：带取消功能
 */
async function example3WithCancel() {
    const file = document.getElementById('file-input').files[0];
    const eventID = 'your-event-id';
    const uploader = new ChunkUploader(file, eventID);

    // 绑定取消按钮
    document.getElementById('cancel-btn').onclick = async () => {
        await uploader.cancel();
        alert('上传已取消');
    };

    try {
        await uploader.upload((progress) => {
            console.log(`进度: ${progress}%`);
        });
    } catch (error) {
        if (error.message.includes('cancelled')) {
            console.log('用户取消了上传');
        } else {
            console.error('上传失败:', error);
        }
    }
}

/**
 * 示例4：React Hook 封装
 */
function useChunkUpload(file, eventID) {
    const [progress, setProgress] = React.useState(0);
    const [status, setStatus] = React.useState('idle'); // idle, uploading, completed, error
    const [error, setError] = React.useState(null);
    const uploaderRef = React.useRef(null);

    const startUpload = React.useCallback(async () => {
        if (!file) return;

        setStatus('uploading');
        setProgress(0);
        setError(null);

        uploaderRef.current = new ChunkUploader(file, eventID);

        try {
            const filePath = await uploaderRef.current.upload((prog) => {
                setProgress(prog);
            });

            setStatus('completed');
            return filePath;
        } catch (err) {
            setError(err.message);
            setStatus('error');
            throw err;
        }
    }, [file, eventID]);

    const cancelUpload = React.useCallback(async () => {
        if (uploaderRef.current) {
            await uploaderRef.current.cancel();
            setStatus('idle');
            setProgress(0);
        }
    }, []);

    const resumeUpload = React.useCallback(async () => {
        if (!uploaderRef.current) return;

        setStatus('uploading');

        try {
            const filePath = await uploaderRef.current.resume((prog) => {
                setProgress(prog);
            });

            setStatus('completed');
            return filePath;
        } catch (err) {
            setError(err.message);
            setStatus('error');
            throw err;
        }
    }, []);

    return {
        progress,
        status,
        error,
        startUpload,
        cancelUpload,
        resumeUpload,
    };
}

/**
 * 示例5：Vue Composition API 封装
 */
function useChunkUploadVue(file, eventID) {
    const progress = Vue.ref(0);
    const status = Vue.ref('idle');
    const error = Vue.ref(null);
    let uploader = null;

    const startUpload = async () => {
        if (!file.value) return;

        status.value = 'uploading';
        progress.value = 0;
        error.value = null;

        uploader = new ChunkUploader(file.value, eventID.value);

        try {
            const filePath = await uploader.upload((prog) => {
                progress.value = prog;
            });

            status.value = 'completed';
            return filePath;
        } catch (err) {
            error.value = err.message;
            status.value = 'error';
            throw err;
        }
    };

    const cancelUpload = async () => {
        if (uploader) {
            await uploader.cancel();
            status.value = 'idle';
            progress.value = 0;
        }
    };

    const resumeUpload = async () => {
        if (!uploader) return;

        status.value = 'uploading';

        try {
            const filePath = await uploader.resume((prog) => {
                progress.value = prog;
            });

            status.value = 'completed';
            return filePath;
        } catch (err) {
            error.value = err.message;
            status.value = 'error';
            throw err;
        }
    };

    return {
        progress,
        status,
        error,
        startUpload,
        cancelUpload,
        resumeUpload,
    };
}
