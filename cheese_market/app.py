from flask import Flask, request, jsonify
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart

app = Flask(__name__)

SMTP_SERVER = "smtp-mail.outlook.com"
SMTP_PORT = 587
SMTP_USER = "230771@astanait.edu.kz"
SMTP_PASSWORD = "Wux61632"

@app.route("/send_email", methods=["POST"])
def send_email():
    data = request.get_json()
    if not data:
        return jsonify({"error": "Invalid input"}), 400

    to_email = data.get("to")
    subject = data.get("subject")
    body = data.get("body")

    if not to_email or not subject or not body:
        return jsonify({"error": "Missing fields: 'to', 'subject', 'body'"}), 400

    try:
        message = MIMEMultipart()
        message["From"] = SMTP_USER
        message["To"] = to_email
        message["Subject"] = subject
        message.attach(MIMEText(body, "plain"))

        with smtplib.SMTP(SMTP_SERVER, SMTP_PORT) as server:
            server.starttls()
            server.login(SMTP_USER, SMTP_PASSWORD)
            server.sendmail(SMTP_USER, to_email, message.as_string())
        return jsonify({"status": "Email sent successfully"}), 200
    except Exception as e:
        return jsonify({"error": str(e)}), 500

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8081)