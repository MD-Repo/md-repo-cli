#! /usr/bin/python3
import base64, struct, sys
from Crypto.Cipher import AES
from Crypto.Util.Padding import pad

def encode_ticket_submission_string(hashed_password, payload):
    # Obtain the user's hashed password
    
    # Pad the key to 16 bytes and append "671ff9e1f816451b"
    key = (str(hashed_password)+ "671ff9e1f816451b")[:16].encode('utf-8')
    print(key)

    # Set the custom IV
    iv = b"4e2f34041d564ed8"

    # Convert payload to bytes
    payload_to_bytes = payload.encode('utf-8')

    # Get the content length
    content_length = len(payload_to_bytes)

    # Encrypt the payload using AES in CBC mode
    cipher = AES.new(key, AES.MODE_CBC, iv=iv)
    ciphertext = cipher.encrypt(pad(payload_to_bytes, AES.block_size))

    # Prepend the content length as 4 bytes in little endian format
    content_length_bytes = struct.pack('<I', content_length)
    ciphertext = content_length_bytes + ciphertext

    # Encode the ciphertext using base64
    encoded_ciphertext = base64.b64encode(ciphertext)

    return encoded_ciphertext

def main():
    print(encode_ticket_submission_string(sys.argv[1], sys.argv[2]))

if __name__ == "__main__":
    main()