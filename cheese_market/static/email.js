document.addEventListener('DOMContentLoaded', function () {
    const form = document.getElementById('emailForm');
    form.addEventListener('submit', async (event) => {
        event.preventDefault();

        const to = document.getElementById('to').value;
        const subject = document.getElementById('subject').value;
        const message = document.getElementById('message').value;
        const fileInput = document.getElementById('file');
        const file = fileInput.files[0];
        const fileContent = await readFile(file);

        const payload = { 
            to, 
            subject, 
            body: message, 
            file: {
                filename: file.name,
                content: fileContent
            }
        };

        fetch('/send_email', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        })
        .then(response => response.json())
        .then(result => {
            if (result.message) {
                alert(result.message);  // Успех
            } else if (result.error) {
                alert(`Error: ${result.error}`);  // Ошибка
            } else {
                alert('Unexpected response format');
            }
        })
        .catch(error => {
            alert(`Request failed: ${error}`);
        });
        
    });

    function readFile(file) {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onload = () => resolve(reader.result);
            reader.onerror = reject;
            reader.readAsDataURL(file);
        });
    }
});
