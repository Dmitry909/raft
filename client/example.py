
import subprocess
import time

# Путь к бинарному файлу сервера
binary_path = "../server/main"

# Аргументы для бинарного файла, если они нужны
args = ["8000", "8000", "500"]

# Запускаем процесс
process = subprocess.Popen([binary_path] + args,
                           stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE)

# Теперь процесс запущен и работает независимо от питон-программы.
print(f"Запущен серверный процесс с PID: {process.pid}")
time.sleep(10)
# Ваш основной код может продолжать работать и завершаться по необходимости
# процесс будет работать до тех пор, пока не завершится ваша программа или
# не будет убит вручную.