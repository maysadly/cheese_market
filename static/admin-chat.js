let activeChats = [];
let currentChatID = null;
const socket = new WebSocket('ws://localhost:8080/ws');

// Загрузка активных чатов при старте
window.onload = function () {
    loadActiveChats();
};

// Получение списка активных чатов
async function loadActiveChats() {
    try {
        const response = await fetch('/api/active-chats');
        const data = await response.json();
        activeChats = data;
        renderActiveChats();
    } catch (error) {
        console.error("Ошибка загрузки активных чатов:", error);
    }
}

// Отображение списка активных чатов
function renderActiveChats() {
    const chatListDiv = document.getElementById('activeChats');
    chatListDiv.innerHTML = '';

    activeChats.forEach(chat => {
        const chatItem = document.createElement('div');
        chatItem.className = 'chat-item';
        chatItem.textContent = `Чат #${chat.chat_id}`;
        chatItem.onclick = () => openChat(chat.chat_id);
        chatListDiv.appendChild(chatItem);
    });
}

// Открытие чата
function openChat(chatID) {
    currentChatID = chatID;
    document.getElementById('chatBox').style.display = 'block';
    document.getElementById('chatMessages').innerHTML = '';
    loadChatHistory();
}

// Отправка сообщения администратором
function sendMessage() {
    const input = document.getElementById('chatInput');
    if (!input.value.trim()) return;

    const messageData = {
        type: "send_message",
        chat_id: currentChatID,
        sender: "admin",
        content: input.value.trim()
    };

    socket.send(JSON.stringify(messageData));

    // Удаляем дублирование (раньше здесь был addMessageToUI)
    input.value = '';

    // Загружаем обновлённую историю чата после отправки
    setTimeout(loadChatHistory, 300);
}

// Обработка входящих сообщений WebSocket
socket.onmessage = function (event) {
    console.log("Получено сообщение через WebSocket:", event.data);
    const data = JSON.parse(event.data);

    switch (data.type) {
        case 'chat_created':
            console.log("Чат создан с ID:", data.chat_id);
            currentChatID = data.chat_id;
            document.getElementById('chatID').textContent = currentChatID;
            document.getElementById('chatWindow').style.display = 'block';
            document.getElementById('chatControl').style.display = 'none';
            break;

        case 'new_message':
            console.log("Новое сообщение:", data);
            if (data.chat_id === currentChatID) {
                // Вместо прямого добавления сообщения, загружаем историю
                loadChatHistory();
            }
            break;
    }
};

// Загрузка истории чата
async function loadChatHistory() {
    if (!currentChatID) {
        console.error("Ошибка: currentChatID отсутствует!");
        return;
    }

    try {
        const response = await fetch(`/api/chat-history?chat_id=${currentChatID}`);
        const data = await response.json();

        if (Array.isArray(data)) {
            const messagesDiv = document.getElementById('chatMessages');
            messagesDiv.innerHTML = ''; // Очищаем чат перед обновлением
            data.forEach(msg => addMessageToUI(msg.sender, msg.content));
        } else {
            console.error("Ошибка: Ожидался массив, получено:", data);
        }
    } catch (error) {
        console.error("Ошибка загрузки истории чата:", error);
    }
}

// Добавление сообщений в UI (без дубликатов)
function addMessageToUI(sender, message) {
    const messagesDiv = document.getElementById('chatMessages');

    const messageDiv = document.createElement('div');
    messageDiv.className = `message ${sender}`;
    messageDiv.innerHTML = `<strong>${sender}:</strong> ${message}`;
    messagesDiv.appendChild(messageDiv);
}

// Закрытие чата
async function closeChat() {
    if (!currentChatID) {
        console.error("Ошибка: currentChatID отсутствует!");
        return;
    }

    if (socket.readyState !== WebSocket.OPEN) {
        console.error("Ошибка: WebSocket соединение закрыто!");
        return;
    }

    console.log("Отправка запроса на закрытие чата...");
    
    socket.send(JSON.stringify({
        type: "close_chat",
        chat_id: currentChatID
    }));
}
