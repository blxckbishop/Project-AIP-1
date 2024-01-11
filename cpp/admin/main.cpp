#include <fstream>
#include <xlnt/xlnt.hpp>
#include <nlohmann/json.hpp>
#include <iostream>
#include <string>
#define CPPHTTPLIB_OPENSSL_SUPPORT
#include <codecvt>
#include <httplib.h>
#include <regex>
#include "sheet1.cpp"
#include "sheet2.cpp"
#include <locale>
#include <jwt-cpp/jwt.h>

using namespace httplib;
using namespace nlohmann;

std::mutex settingsFileMutex;
std::mutex settingsMutex;
std::condition_variable settingsUpdated;
std::atomic<bool> restartDownloadFlag(false);

const std::string SECRET_ADMIN_JWT = "123a";
std::map<std::string, std::pair<int64_t, int64_t>> requestTokensMap;

std::string basePath = "C:/Users/VLAD/CLionProjects/admin7/";

// UTILS ///////////////////////////////////////////////////
//

// cookie - "token=weoifoierwubbehiruwbeghiruwforebgreg;time_exp=12312432;"
// name - tokenregreg
std::string getTokenFromCookie(std::string name, std::string cookie) {
    size_t namePos = cookie.find(name + "=");
    if (namePos != std::string::npos) {
        namePos += name.length() + 1;
        size_t endPos = cookie.find_first_of("; ", namePos);
        if (endPos != std::string::npos) {
            return cookie.substr(namePos, endPos - namePos);
        } else {
            return cookie.substr(namePos);
        }
    }
    return "";
}

// isValidJWT - валидирует jwt token
std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
        jwt::traits::kazuho_picojson>::basic_claim_t>> isValidJWT(std::string jwt) {
    try {
        auto decoded_token = jwt::decode(jwt);
        // подпись - хеш -
        auto verifier = jwt::verify().allow_algorithm(jwt::algorithm::hs256{SECRET_ADMIN_JWT});
        verifier.verify(decoded_token);
        auto payload = decoded_token.get_payload_claims();
        auto expires_at = payload["exp"].as_int();
        auto current_time = std::chrono::system_clock::now().time_since_epoch().count() / 1000000000;
        if (expires_at < current_time) {
            return std::make_pair(
                    false,
                    std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
                            jwt::traits::kazuho_picojson>::basic_claim_t>());
        }
        return std::make_pair(true, payload);
    } catch (const std::exception &e) {
        return std::make_pair(
                false,
                std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
                        jwt::traits::kazuho_picojson>::basic_claim_t>());
    }
}
// jwt генератор
std::string generateJWT(int64_t chatId) {
    auto tokeExpiresAt = std::chrono::system_clock::now() + std::chrono::minutes(15);
    return jwt::create()
            .set_type("JWT")
            .set_payload_claim("chat_id", jwt::claim(picojson::value(chatId)))
            .set_payload_claim("exp", jwt::claim(tokeExpiresAt))
            .sign(jwt::algorithm::hs256{SECRET_ADMIN_JWT});
}
// генератор рандомного кода
std::string generateRandomCode(int length) {
    std::string characters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
    std::string result;

    std::mt19937 generator(static_cast<unsigned int>(std::time(0)));
    std::uniform_int_distribution<int> distribution(0, characters.size() - 1);

    for (int i = 0; i < length; ++i) {
        result += characters[distribution(generator)];
    }

    return result;
}

// логи загрузок
void logToFile(const std::string &message) {
    std::ofstream logfile(basePath + "downloadLogs.log", std::ios_base::app);
    if (logfile.is_open()) {
        time_t now = time(0);
        struct tm *timeinfo = localtime(&now);
        char buffer[80];
        strftime(buffer, sizeof(buffer), "%Y-%m-%d %H:%M:%S", timeinfo);

        logfile << "[" << buffer << "] " << message << std::endl;
        logfile.close();
    } else {
        std::cerr << "Error opening log file!" << std::endl;
    }
}

// вычисляет хеш файла
std::string calculateFileHash(const std::string &filePath) {
    // Открываем файл в бинарном режиме
    std::ifstream file(filePath, std::ios::binary);

    if (!file.is_open()) {
        std::cerr << "Unable to open file: " << filePath << std::endl;
        return "";
    }

    // Получаем размер файла
    file.seekg(0, std::ios::end);
    std::streampos fileSize = file.tellg();
    file.seekg(0, std::ios::beg);

    // Выделяем буфер для чтения файла
    std::vector<char> buffer(fileSize);

    // Читаем файл в буфер
    file.read(buffer.data(), fileSize);

    // Закрываем файл
    file.close();

    // Инициализируем контекст SHA-256
    SHA256_CTX sha256Context;
    SHA256_Init(&sha256Context);

    // Обновляем контекст хэширования данными из буфера
    SHA256_Update(&sha256Context, buffer.data(), fileSize);

    // Получаем итоговый хэш
    unsigned char hash[SHA256_DIGEST_LENGTH];
    SHA256_Final(hash, &sha256Context);

    // Преобразуем байты хэша в строку
    std::stringstream hashString;
    for (int i = 0; i < SHA256_DIGEST_LENGTH; ++i) {
        hashString << std::hex << std::setw(2) << std::setfill('0') << static_cast<int>(hash[i]);
    }

    return hashString.str();
}
// для определения числа
std::string getCurrentDateTime() {
    auto currentTime = std::chrono::system_clock::to_time_t(std::chrono::system_clock::now());
    struct std::tm *timeInfo = std::localtime(&currentTime);
    std::stringstream ss;
    ss << std::put_time(timeInfo, "%Y-%m-%d %H:%M:%S");
    return ss.str();
}

std::string toLowerCase(const std::string &input) {
    std::wstring_convert<std::codecvt_utf8<wchar_t> > converter;
    std::wstring wide_input = converter.from_bytes(input);

    std::wstring result;
    for (wchar_t ch: wide_input) {
        result += towlower(ch);
    }

    std::wstring_convert<std::codecvt_utf8<wchar_t> > utf8_converter;
    return utf8_converter.to_bytes(result);
}
// приведение к нормальному виду (из относительных в реальные т.е из пн,вт,ср в числа)
json timetableNormalization(const json &inputSchedule) {
    json outputSchedule;

    for (const auto &course: inputSchedule) {
        const std::string &startDate = course["startDate"];
        const std::string &endDate = course["endDate"];
        const std::string &group = course["group"];

        std::istringstream startDateStream(startDate);
        std::tm startDateStruct = {};
        startDateStream >> std::get_time(&startDateStruct, "%d.%m.%y");

        std::istringstream endDateStream(endDate);
        std::tm endDateStruct = {};
        endDateStream >> std::get_time(&endDateStruct, "%d.%m.%y");

        json items = course["items"];
        bool isEvenWeek = false;

        for (std::tm currentDateStruct = startDateStruct;
             std::difftime(std::mktime(&currentDateStruct), std::mktime(&endDateStruct)) <= 0;
             currentDateStruct.tm_mday++) {
            int dayOfWeek = (currentDateStruct.tm_wday ? currentDateStruct.tm_wday : 7);

            json items = course["items"];
            for (const auto &item: items) {
                if (item["dayOfWeek"] == std::to_string(dayOfWeek) && item["isEvenWeek"] == isEvenWeek && item["title"]
                                                                                                          != "" && item["type"] != "" && item["teacher"] != "") {
                    std::ostringstream formattedDate;
                    formattedDate << std::put_time(&currentDateStruct, "%d.%m.%y");

                    json scheduleItem = {
                            {"address", item.at("address")},
                            {"comment", item.at("comment")},
                            {"date", formattedDate.str()},
                            {"group", item.at("group")},
                            {"lesson_number", item.at("lessonNumber")},
                            {"teacher", item.at("teacher")},
                            {"title", item.at("title")},
                            {"type", item.at("type")}
                    };
                    outputSchedule.push_back(scheduleItem);
                };
            }
            if (dayOfWeek == 7) isEvenWeek = !isEvenWeek;
        }
    }
    return outputSchedule;
}

// парсилка имени листа
void parseTitleSheetToJson(const std::string &input, json &output) {
    std::regex pattern("курс (\\d+) (\\S+) \\((\\d{2}\\.\\d{2}\\.\\d{2})-(\\d{2}\\.\\d{2}\\.\\d{2})\\)");
    std::smatch matches;
    if (std::regex_search(input, matches, pattern)) {
        output["courseNumber"] = matches[1];
        output["group"] = matches[2];
        output["startDate"] = matches[3];
        output["endDate"] = matches[4];
    } else {
        std::cerr << "Неверный формат строки" << std::endl;
    }
}

// парсилка листа из эксель
json parseSheet(xlnt::worksheet ws, int columnCount, const int sheetMarking[]) {
    std::map<std::string, std::string> dayOfWeekMap = {
            {"понедельник", "1"},
            {"вторник", "2"},
            {"среда", "3"},
            {"четверг", "4"},
            {"пятница", "5"},
            {"суббота", "6"},
            {"воскресенье", "7"}
    };

    json json_array;
    size_t col, row;
    int col_nech = 0, row_day = 0;
    for (int i = 0; i < columnCount; i++) {
        col = sheetMarking[i * 4];
        row = sheetMarking[i * 4 + 1]; //D5
        if (col > 12) { col_nech = 13; } else { col_nech = 1; }; //chet, nechet
        if ((row >= 5) && (row <= 34)) { row_day = 5; }; //ponedelnik
        if ((row >= 35) && (row <= 64)) { row_day = 35; }; //vtornik
        if ((row >= 65) && (row <= 94)) { row_day = 65; }; //sreda
        if ((row >= 95) && (row <= 124)) { row_day = 95; }; //chetverg
        if ((row >= 125) && (row <= 154)) { row_day = 125; }; //pyatniza

        json json_obj;
        json_obj["group"] = ws.cell(col + sheetMarking[i * 4 + 2], 4).to_string();
        std::string weekType = ws.cell(col_nech, 1).to_string();
        json_obj["isEvenWeek"] = "четная неделя" == toLowerCase(weekType);
        std::string week = ws.cell(col_nech, row_day).to_string();
        json_obj["dayOfWeek"] = dayOfWeekMap[toLowerCase(week)];

        json_obj["lessonNumber"] = std::to_string((ws.cell(col_nech + 1, row).to_string().at(0) - 48));
        json_obj["type"] = ws.cell(col + sheetMarking[i * 4 + 3], row).to_string();
        json_obj["title"] = ws.cell(col, row).to_string();
        json_obj["teacher"] = ws.cell(col, row + 1).to_string();
        json_obj["address"] = ws.cell(col, row + 2).to_string();
        json_obj["comment"] = "";
        json_array.push_back(json_obj);
    }
    return json_array;
}

// функция, которая вызывает парсилку чтобы спарсить листы с расписанием и экзаменами
json parseTimetableFile(std::string path) {
    xlnt::workbook wb;
    wb.load(path);
    json json_main;

    json json_classes_list1;
    auto ws1 = wb.sheet_by_index(0);
    parseTitleSheetToJson(ws1.title(), json_classes_list1);
    json_classes_list1["items"] = parseSheet(ws1, 360, sheet1);
    json_classes_list1["addedDateTime"] = getCurrentDateTime();
    json_main.push_back(json_classes_list1);

    json json_classes_list2;
    auto ws2 = wb.sheet_by_index(1);
    parseTitleSheetToJson(ws2.title(), json_classes_list2);
    json_classes_list2["items"] = parseSheet(ws2, 180, sheet2);
    json_classes_list2["addedDateTime"] = getCurrentDateTime();
    json_main.push_back(json_classes_list2);
    return json_main;
}

// ROUTES ///////////////////////////////////////////////////

// роут по которому получаешь инфу из файла с логами для админки
// проверяет что в куках правильный jwt, который принадлежит админ серверу
void getDownloadLogsHandler(const Request &req, Response &res) {
    auto cookieToken = req.get_header_value("Cookie");
    std::string tokenValue = getTokenFromCookie("token", cookieToken);
    std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
            jwt::traits::kazuho_picojson>::basic_claim_t> > jwtPair = isValidJWT(tokenValue);
    if (jwtPair.first) {
        std::string newJWT = generateJWT(jwtPair.second["chat_id"].as_int());
        res.set_header("Set-Cookie", "token=" + newJWT);
        std::lock_guard<std::mutex> lock(settingsFileMutex);
        try {
            std::ifstream file(basePath + "downloadLogs.log");

            if (!file.is_open()) {
                res.status = 500;
                res.set_content("Error reading configuration file", "text/plain");
                return;
            }

            std::stringstream buffer;
            buffer << file.rdbuf();
            std::string fileContent = buffer.str();
            res.status = 200;
            res.set_content(fileContent, "text/plain");
        } catch (const std::exception &e) {
            res.status = 500;
            res.set_content("Internal Error", "text/plain");
        }
    } else {
        res.status = 403;
        res.set_content("Forbidden", "text/plain");
    }
}

//  устанавливает конфиг, достает из json. тк это post
void setAutomaticUpdateConfigHandler(const Request &req, Response &res) {
    auto cookieToken = req.get_header_value("Cookie");
    std::string tokenValue = getTokenFromCookie("token", cookieToken);
    std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
            jwt::traits::kazuho_picojson>::basic_claim_t> > jwtPair = isValidJWT(tokenValue);
    if (jwtPair.first) {
        std::string newJWT = generateJWT(jwtPair.second["chat_id"].as_int());
        res.set_header("Set-Cookie", "token=" + newJWT);
        std::lock_guard<std::mutex> lock(settingsFileMutex);
        try {
            json body = json::parse(req.body);
            std::string url = body["url"];
            std::string delay = body["delay"];
            bool enable = body["enable"];

            json jsonData = {
                    {"url", url},
                    {"delay", delay},
                    {"enable", enable}
            };

            std::ofstream file(basePath + "automaticUpdateConfig.json");
            file << std::setw(4) << jsonData << std::endl;
            file.close();
            res.status = 200;
            res.set_content("Config successfully updated", "text/plain");
        } catch (const std::exception &e) {
            res.status = 500;
            res.set_content("Internal Error", "text/plain");
        }
    } else {
        res.status = 403;
        res.set_content("Forbidden", "text/plain");
    }
}

// обновление конфига для скачивания
void getAutomaticUpdateConfigHandler(const Request &req, Response &res) {
    auto cookieToken = req.get_header_value("Cookie");
    std::string tokenValue = getTokenFromCookie("token", cookieToken);
    std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
            jwt::traits::kazuho_picojson>::basic_claim_t> > jwtPair = isValidJWT(tokenValue);
    if (jwtPair.first) {
        std::string newJWT = generateJWT(jwtPair.second["chat_id"].as_int());
        res.set_header("Set-Cookie", "token=" + newJWT);
        std::lock_guard<std::mutex> lock(settingsFileMutex);
        try {
            std::ifstream file(basePath + "automaticUpdateConfig.json");

            if (!file.is_open()) {
                res.status = 500;
                res.set_content("Error reading configuration file", "text/plain");
                return;
            }

            json jsonData;
            file >> jsonData;
            file.close();

            res.status = 200;
            res.set_content(jsonData.dump(), "application/json");
        } catch (const std::exception &e) {
            res.status = 500;
            res.set_content("Internal Error", "text/plain");
        }
    } else {
        res.status = 403;
        res.set_content("Forbidden", "text/plain");
    }
}
// установление нового расписания для модуля расписания
void setTimetableHandler(const Request &req, Response &res) {
    auto cookieToken = req.get_header_value("Cookie");
    std::string tokenValue = getTokenFromCookie("token", cookieToken);
    std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
            jwt::traits::kazuho_picojson>::basic_claim_t> > jwtPair = isValidJWT(tokenValue);
    if (jwtPair.first) {
        std::string newJWT = generateJWT(jwtPair.second["chat_id"].as_int());
        res.set_header("Set-Cookie", "token=" + newJWT);

        json response;
        response["status"] = "error";

        if (req.has_file("file")) {
            auto file = req.get_file_value("file");
            std::string temp_path = basePath + "temp/" + file.filename;
            std::ofstream outfile(temp_path, std::ofstream::binary);
            outfile.write(file.content.data(), file.content.size());
            outfile.close();
            json main = timetableNormalization(parseTimetableFile(temp_path));
            std::string json_data = main.dump();

            httplib::Client client("http://localhost:8050");

            // для дебага
            json main2 = parseTimetableFile(temp_path);
            std::ofstream ofile(basePath + "info.txt", std::ios::out);
            ofile << main2.dump(4) << std::endl;
            json mainNorm = timetableNormalization(main2);

            std::ofstream dofile(basePath + "infoNormalized.txt", std::ios::out);
            dofile << mainNorm.dump(4) << std::endl;
            // konec

            auto res2 = client.Post("/setTimetable", json_data, "application/json");
            if (res2 && res2->status == 200) {
                response["status"] = "success";
                response["message"] = "File uploaded successfully";
                res.set_content(response.dump(), "application/json");
            } else {
                response["message"] = "Schedule server error";
                res.set_content(response.dump(), "application/json");
            }
        } else {
            response["message"] = "No file uploaded";
            res.set_content(response.dump(), "application/json");
        }
    } else {
        res.status = 403;
        res.set_content("Forbidden", "text/plain");
    }
}
// получение расписания из модуля расписания
void getTimetableHandler(const Request &req, Response &res) {
    auto cookieToken = req.get_header_value("Cookie");
    std::string tokenValue = getTokenFromCookie("token", cookieToken);
    std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
            jwt::traits::kazuho_picojson>::basic_claim_t> > jwtPair = isValidJWT(tokenValue);
    if (jwtPair.first) {
        std::string newJWT = generateJWT(jwtPair.second["chat_id"].as_int());
        res.set_header("Set-Cookie", "token=" + newJWT);

        httplib::Client client("http://localhost:8050");
        auto result = client.Get("/getTimetable");
        if (result) {
            res.set_content(result->body, "application/json");
        } else {
            res.status = 500;
            res.set_content("Failed to fetch data from http://localhost:8050/getTimetable", "text/plain");
        }
    } else {
        res.status = 403;
        res.set_content("Forbidden", "text/plain");
    }
}

void changeRolesHandler(const Request &req, Response &res) {
    auto cookieToken = req.get_header_value("Cookie");
    std::string tokenValue = getTokenFromCookie("token", cookieToken);
    std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
            jwt::traits::kazuho_picojson>::basic_claim_t> > jwtPair = isValidJWT(tokenValue);
    if (jwtPair.first) {
        std::string newJWT = generateJWT(jwtPair.second["chat_id"].as_int());
        res.set_header("Set-Cookie", "token=" + newJWT);
        json response;
        response["status"] = "error";
        try {
            json body = json::parse(req.body);
            std::int64_t chatId = body["chat_id"];
            std::vector<std::string> roles = body["roles"];
            httplib::Client client("http://localhost:8090");
            json request;
            request["chat_id"] = chatId;
            request["roles"] = body["roles"];

            auto res2 = client.Post("/changeRoles", request.dump(), "application/json");

            if (res2 && res2->status == 200) {
                response["status"] = "success";
                response["message"] = "User deleted successfully";

                res.set_content(response.dump(), "application/json");
            } else {
                res.status = 500;
                response["message"] = "Error sending request to deleteUser in Go server";

                res.set_content(response.dump(), "application/json");
            }
        } catch (const std::exception &e) {
            response["message"] = e.what();
            res.set_content(response.dump(), "application/json");
        }
    } else {
        res.status = 403;
        res.set_content("Forbidden", "text/plain");
    }
}

void deleteUserHandler(const Request &req, Response &res) {
    auto cookieToken = req.get_header_value("Cookie");
    std::string tokenValue = getTokenFromCookie("token", cookieToken);
    std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
            jwt::traits::kazuho_picojson>::basic_claim_t> > jwtPair = isValidJWT(tokenValue);
    if (jwtPair.first) {
        std::string newJWT = generateJWT(jwtPair.second["chat_id"].as_int());
        res.set_header("Set-Cookie", "token=" + newJWT);

        json response;
        response["status"] = "error";
        try {
            json requestJson = json::parse(req.body);
            std::int64_t chatId = requestJson["chat_id"];
            httplib::Client client("http://localhost:8090");
            json request;
            request["chat_id"] = chatId;

            auto res2 = client.Post("/deleteUser", request.dump(), "application/json");

            if (res2 && res2->status == 200) {
                response["status"] = "success";
                response["message"] = "User deleted successfully";
                res.set_content(response.dump(), "application/json");
            } else {
                res.status = 500;
                response["message"] = "Error sending request to deleteUser in Go server";
                res.set_content(response.dump(), "application/json");
            }
        } catch (const json::exception &e) {
            res.status = 400;
            response["message"] = "Error parsing JSON payload";
            res.set_content(response.dump(), "application/json");
        } catch (const std::exception &e) {
            res.status = 500;
            response["message"] = "Internal server error";
            res.set_content(response.dump(), "application/json");
        }
    } else {
        res.status = 403;
        res.set_content("Forbidden", "text/plain");
    }
}

void getAllUsersHandler(const Request &req, Response &res) {
    auto cookieToken = req.get_header_value("Cookie");
    std::string tokenValue = getTokenFromCookie("token", cookieToken);
    std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
            jwt::traits::kazuho_picojson>::basic_claim_t> > jwtPair = isValidJWT(tokenValue);
    if (jwtPair.first) {
        std::string newJWT = generateJWT(jwtPair.second["chat_id"].as_int());
        res.set_header("Set-Cookie", "token=" + newJWT);
        httplib::Client client("http://localhost:8090");
        auto result = client.Get("/getAllUsers");
        if (result) {
            res.set_content(result->body, "application/json");
        } else {
            res.status = 500;
            res.set_content("Failed to fetch data from http://localhost:8090/getAllUsers", "text/plain");
        }
    } else {
        res.status = 403;
        res.set_content("Forbidden", "text/plain");
    }
}
// проверка реквест токена на валидность
bool isRequestTokenValid(const std::string &requestToken) {
    auto it = requestTokensMap.find(requestToken);
    if (it != requestTokensMap.end()) {
        auto currentTimeInSeconds = std::chrono::duration_cast<std::chrono::seconds>(
                std::chrono::system_clock::now().time_since_epoch()
        ).count() / 100;

        if (it->second.first > currentTimeInSeconds && it->second.second >= 0) {
            return true;
        } else {
            return false;
        }
    } else {
        return false;
    }
}
// Основная html
void setupAdminHtml(Response &res) {
    res.set_header("Content-Type", "text/html; charset=utf-8");
    std::ifstream file(basePath + "html/index.html");
    if (file.is_open()) {
        std::stringstream buffer;
        buffer << file.rdbuf();
        res.set_content(buffer.str(), "text/html; charset=utf-8");
    } else {
        res.set_content("Error: Could not open index.html", "text/plain");
    }
}
// дефолтная html
void setupInviteHtml(Response &res, std::string requestToken) {
    std::string html = R"(
        <!DOCTYPE html>
        <html>
        <head>
            <title>Invite Setup</title>
        </head>
        <body>
            <h1>Welcome to Schedule Project</h1>
            <p>Please enter the following code in our Telegram bot:</p>
            <code>[Your Code]: <strong>)" + requestToken + R"(</strong></code>
            <p>Visit our Telegram bot: <a href="https://t.me/ScheduleProjectBot" target="_blank">Schedule Project Bot</a></p>
        </body>
        </html>
    )";
    res.set_content(html, "text/html");
}

void getAdminHandler(const Request &req, Response &res) {
    auto requestToken = req.get_param_value("request_token");
    auto cookieToken = req.get_header_value("Cookie");

    if (!requestToken.empty() && isRequestTokenValid(requestToken)) {
        std::string newJWT = generateJWT(requestTokensMap[requestToken].second);
        res.set_header("Set-Cookie", "token=" + newJWT);
        requestTokensMap[requestToken] = std::make_pair(-1, -1);
        setupAdminHtml(res);
    } else {
        if (!requestToken.empty()) {
            std::string tokenValue = getTokenFromCookie("token", cookieToken);
            std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
                    jwt::traits::kazuho_picojson>::basic_claim_t> > jwtPair = isValidJWT(tokenValue);

            if (tokenValue.length() != 0 && jwtPair.first) {
                setupAdminHtml(res);
                std::string newJWT = generateJWT(jwtPair.second["chat_id"].as_int());
                res.set_header("Set-Cookie", "token=" + newJWT);
            } else {
                auto it = requestTokensMap.find(requestToken);
                if (it != requestTokensMap.end()) {
                    setupInviteHtml(res, requestToken);
                } else {
                    std::string redirect_url = "/admin";
                    res.set_redirect(redirect_url.c_str());
                }
            }
        } else {
            std::string requestToken = generateRandomCode(4);
            requestTokensMap[requestToken] = std::make_pair(-1, -1);
            std::string redirect_url = "/admin?request_token=" + requestToken;
            res.set_redirect(redirect_url.c_str());
        }
    }
}

// для старта админки
void startAdminSessionHandler(const Request &req, Response &res) {
    std::string jwt = req.get_param_value("token");
    std::string requestToken = req.get_param_value("request_token");

    if (jwt.empty()) {
        res.status = 400; // Bad Request
        res.set_content("Missing 'token' parameter", "text/plain");
        return;
    }

    std::pair<bool, std::unordered_map<jwt::traits::kazuho_picojson::string_type, jwt::decoded_jwt<
            jwt::traits::kazuho_picojson>::basic_claim_t>> jwtPair = isValidJWT(jwt);
    if (!jwtPair.first) {
        res.status = 400;
        res.set_content("jwt not valid", "text/plain");
        return;
    }

    auto payload = jwtPair.second;
    auto chatId = payload["chat_id"].as_int();
    auto expires_at = payload["exp"].as_int();
    auto current_time = std::chrono::system_clock::now().time_since_epoch().count() / 1000000000;
    if (expires_at < current_time) {
        res.set_content("Token has expired", "text/plain");
        return;
    }

    std::string action = payload["action"].as_string();
    if (!action.empty()) {
        if (action == "AdminSessionStart" && requestToken.empty()) {
            std::string requestToken = generateRandomCode(4);
            auto current_time = std::chrono::system_clock::now().time_since_epoch().count() / 1000000000;
            requestTokensMap[requestToken] = std::make_pair(current_time + 5 * 60, chatId);
            res.set_content("http://localhost:8060/admin?request_token=" + requestToken, "text/plain");
            return;
        } else if (action == "AdminSessionStartToken" && !requestToken.empty()) {
            auto current_time = std::chrono::system_clock::now().time_since_epoch().count() / 1000000000;
            if (requestTokensMap.find(requestToken) != requestTokensMap.end()) {
                std::pair<int, int> values = requestTokensMap[requestToken];
                if (values.first < current_time) {
                    requestTokensMap[requestToken] = std::make_pair(current_time + 5 * 60, chatId);
                    res.set_content("http://localhost:8060/admin?request_token=" + requestToken, "text/plain");
                } else {
                    res.status = 400;
                    res.set_content("Request token expired", "text/plain");
                }
            } else {
                res.status = 400;
                res.set_content("Unknown request token", "text/plain");
            }
            return;
        } else {
            res.status = 400;
            res.set_content("Invalid action", "text/plain");
            return;
        }
    } else {
        res.status = 400;
        res.set_content("Missing 'action' parameter", "text/plain");
        return;
    }
}

void processAutomaticUpdateConfigThreadHandler() {
    // путь то конфига
    std::string filePath = basePath + "automaticUpdateConfig.json";
    std::string previousHash; // прошлый хеш
    while (true) {
        std::string currentHash; // текущий хеш файла
        {
            std::lock_guard<std::mutex> lock(settingsFileMutex); // лочит блок кода { } на мьютексе
            currentHash = calculateFileHash(filePath); // вычисляет хеш файла по filePath
        }
//        std::cout << "--- Поиск изменений currentHash: "<< currentHash << " previousHash: " << previousHash << std::endl;
        if (currentHash != previousHash) { // если прошлый != текущему хешу
            std::cout << "Найдено изменение в конфиге" << std::endl;
            restartDownloadFlag = true;  // переменная которая используется для рестарта скачивания
            settingsUpdated.notify_one();
            previousHash = currentHash;
        }

        std::this_thread::sleep_for(std::chrono::seconds(1));
    }
}

void downloadFileThreadHandler() {
    std::string prevHash = "";
    while (true) {
        json config;
        int delay = 9999;
        try {
            {
                std::lock_guard<std::mutex> lock(settingsFileMutex);
                std::ifstream file(basePath + "automaticUpdateConfig.json");
                file >> config;
                file.close();
            }
            std::string delayStr = to_string(config["delay"]);
            delay = stoi(delayStr.substr(1, delayStr.length() - 2));

            bool enable = config["enable"];
            if (enable) {
                std::string url = config["url"];
                std::string baseUrl = "";
                std::string pathUrl = "";
                std::regex pattern("(https://[^/]+)(/.*)");
                std::smatch matches;
                if (std::regex_match(url, matches, pattern)) {
                    baseUrl = matches[1];
                    pathUrl = matches[2];
                } else {
                    std::cout << "Сопоставление не удалось." << std::endl;
                }

                httplib::Client client(baseUrl);
                auto response = client.Get(pathUrl.c_str());

                std::string filePath;
                if (response && response->status == 200) {
                    std::string tempFolderPath = basePath + "temp/";
                    std::string filename = "downloaded_file.xlsx";
                    filePath = tempFolderPath + filename;

                    std::ofstream outfile(filePath, std::ofstream::binary);
                    outfile.write(response->body.c_str(), response->body.size());
                    outfile.close();

                    std::cout << "Downloaded file saved at: " << filePath << std::endl;

                    std::string currentHash = calculateFileHash(filePath);
                    std::cout << "Файл скачан. совпадает? " <<(currentHash == prevHash) << std::endl;
                    if (currentHash != prevHash) {
                        prevHash = currentHash;
                        json main = timetableNormalization(parseTimetableFile(filePath));
                        std::string json_data = main.dump();

                        httplib::Client client2("http://localhost:8050");
                        auto res2 = client2.Post("/setTimetable", json_data, "application/json");
                        if (res2 && res2->status == 200) {
                            logToFile("Успех");
                        } else {
                            logToFile("Неудача");
                            delay = 5;
                        }
                    }

                } else {
                    std::cerr << "Error downloading file. Status code: " << (response ? response->status : -1) <<
                              std::endl;
                }
            }
        } catch (const std::exception &e) {
            std::cerr << "Exception caught: " << e.what() << std::endl;
            delay = 5;
        }

        std::unique_lock<std::mutex> lock(settingsMutex);
        settingsUpdated.wait_for(lock, std::chrono::minutes(delay), [] { return restartDownloadFlag.load(); });

        if (restartDownloadFlag.load()) {
            restartDownloadFlag = false;
        }
    }
}

int main() {
    // это фигня чтобы не сломался парсинг (dayOfWeek будет "")
    std::setlocale(LC_ALL, "");
    // поток для скачивания файла по ссылке, каждые n минут повторяется цикл, если найдутся изменения, то прервет ожидание потока и зщапустит новый цикл с новым конфигом
    std::thread downloadThread(downloadFileThreadHandler);
    // поток в цикле каждую секунду чекает хеш файла с конфигом, если изменилось с прошлого раза, то уведомляет поток downloadThread о перезаупске
    std::thread processAutomaticUpdateConfigThread(processAutomaticUpdateConfigThreadHandler);

    Server svr;
    svr.Get("/admin", getAdminHandler);

    svr.Get("/getAllUsers", getAllUsersHandler);
    svr.Post("/deleteUser", deleteUserHandler);
    svr.Post("/changeRoles", changeRolesHandler);

    svr.Get("/getTimetable", getTimetableHandler);
    svr.Post("/setTimetable", setTimetableHandler);

    svr.Get("/getAutomaticUpdateConfig", getAutomaticUpdateConfigHandler);
    svr.Post("/setAutomaticUpdateConfig", setAutomaticUpdateConfigHandler);

    svr.Get("/getDownloadLogs", getDownloadLogsHandler);
    svr.Get("/startAdminSession", startAdminSessionHandler);

    std::cout << "Админка слушает..." << std::endl;
    svr.listen("0.0.0.0", 8060);
}
