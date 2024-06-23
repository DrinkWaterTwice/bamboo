

class result:
    def __init__(self, file_path):
        self.file_path = file_path
        self.fasthotstuff_censorship = read_float_array_from_file(file_path + '/fasthotstuff_censorship.txt')
        self.hotstuff_censorship = read_float_array_from_file(file_path + '/hotstuff_censorship.txt')
        self.tchs_censorship = read_float_array_from_file(file_path + '/tchs_censorship.txt')
        self.fasthotstuff_chainQuality = read_float_array_from_file(file_path + '/fasthotstuff_chainQuality.txt')
        self.hotstuff_chainQuality = read_float_array_from_file(file_path + '/hotstuff_chainQuality.txt')
        self.tchs_chainQuality = read_float_array_from_file(file_path + '/tchs_chainQuality.txt')
        self.fasthotstuff_index = read_float_array_from_file(file_path + '/fasthotstuff_index.txt')
        self.hotstuff_index = read_float_array_from_file(file_path + '/hotstuff_index.txt')
        self.tchs_index = read_float_array_from_file(file_path + '/tchs_index.txt')


    
    def get_float_array(self):
        return self.float_array

    def get_file_path(self):
        return self.file_path

def read_float_array_from_file(file_path):
  try:
    with open(file_path, 'r') as file:
      data = file.read().strip().split(',')
      #除去最后为空的部分
      data = data[:800]
      float_array = [float(num) for num in data]
      return float_array
  except FileNotFoundError:
    print(f"File '{file_path}' not found.")
    return []
  except Exception as e:
    print(f"An error occurred while reading the file: {str(e)}")
    return []
  
def main():
  file_path = '101'
  r101 = result(file_path)
  r102 = result('102')
  r72 = result('72')
  print(r101.fasthotstuff_censorship)
main()