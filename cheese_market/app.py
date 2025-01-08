from flask import Flask, request, jsonify
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from email.mime.base import MIMEBase
from email import encoders
import base64
import os
from flask import Flask, request, jsonify
from flask_cors import CORS  # Импортируем CORS

app = Flask(__name__)
CORS(app)

SMTP_SERVER = "smtp-mail.outlook.com"
SMTP_PORT = 587
SMTP_USER = "230771@astanait.edu.kz"
SMTP_PASSWORD = "Wux61632"

@app.route("/send_email", methods=["POST"])
def send_email():
    try:
        # Parse JSON payload
        data = request.get_json()
        if not data:
            return jsonify({"error": "Invalid JSON payload"}), 400

        to_email = data.get("to")
        subject = data.get("subject")
        body = data.get("body")
        file_info = data.get("file", {})

        if not to_email or not subject or not body:
            return jsonify({"error": "Fields 'to', 'subject', and 'body' are required"}), 400

        # Decode the file content (if provided)
        file_content = file_info.get("content")
        filename = file_info.get("filename")
        temp_file_path = None

        if file_content and filename:
            # Decode Base64 file content
            try:
                file_bytes = base64.b64decode(file_content.split(",")[1])
            except Exception:
                return jsonify({"error": "Invalid file content"}), 400

            temp_file_path = f"./{filename}"

            # Save to a temporary file
            with open(temp_file_path, "wb") as f:
                f.write(file_bytes)

        # Create email
        message = MIMEMultipart()
        message["From"] = SMTP_USER
        message["To"] = to_email
        message["Subject"] = subject
        message.attach(MIMEText(body, "plain"))

        # Attach the file if it exists
        if temp_file_path:
            with open(temp_file_path, "rb") as attachment_file:
                attachment = MIMEBase("application", "octet-stream")
                attachment.set_payload(attachment_file.read())
                encoders.encode_base64(attachment)
                attachment.add_header(
                    "Content-Disposition",
                    f"attachment; filename={filename}",
                )
                message.attach(attachment)

        # Send the email
        with smtplib.SMTP(SMTP_SERVER, SMTP_PORT) as server:
            server.starttls()
            server.login(SMTP_USER, SMTP_PASSWORD)
            server.sendmail(SMTP_USER, to_email, message.as_string())

        # Clean up temporary file if it exists
        if temp_file_path and os.path.exists(temp_file_path):
            os.remove(temp_file_path)

        return jsonify({"status": "Email sent successfully"}), 200
    except Exception as e:
        return jsonify({"error": str(e)}), 500


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8081)
